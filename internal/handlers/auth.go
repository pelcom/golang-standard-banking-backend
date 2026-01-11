package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/store"
	"banking/internal/validator"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := validator.ValidateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validator.ValidateEmail(req.Email); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validator.ValidatePassword(req.Password); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to secure password")
		return
	}
	userID := uuid.NewString()
	err = h.txRunner.WithTx(r.Context(), func(tx *sqlx.Tx) error {
		if err := h.users.Create(r.Context(), tx, userID, req.Username, req.Email, passwordHash); err != nil {
			return err
		}
		usdAccountID := uuid.NewString()
		eurAccountID := uuid.NewString()
		if err := h.accounts.Create(r.Context(), tx, usdAccountID, &userID, "USD", 100000, false); err != nil {
			return err
		}
		if err := h.accounts.Create(r.Context(), tx, eurAccountID, &userID, "EUR", 50000, false); err != nil {
			return err
		}
		if err := h.createOpeningBalance(r.Context(), tx, userID, usdAccountID, "USD", 100000); err != nil {
			return err
		}
		if err := h.createOpeningBalance(r.Context(), tx, userID, eurAccountID, "EUR", 50000); err != nil {
			return err
		}
		hasAdmin, err := h.admin.HasAnyAdmin(r.Context())
		if err != nil {
			return err
		}
		if !hasAdmin {
			if err := h.admin.CreateAdmin(r.Context(), tx, userID, true, nil); err != nil {
				return err
			}
		}
		data, _ := json.Marshal(map[string]string{
			"user_id":    userID,
			"ip":         r.RemoteAddr,
			"user_agent": r.UserAgent(),
		})
		return h.audit.Log(r.Context(), tx, userID, "register", "user", userID, string(data))
	})
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code == "23505" {
				respondError(w, http.StatusConflict, "username or email already exists")
				return
			}
		}
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}
	token, err := auth.GenerateToken(h.cfg.JWTSecret, userID, h.cfg.TokenTTL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{
		"token": token,
	})
}

func (h *Handler) createOpeningBalance(ctx context.Context, tx *sqlx.Tx, userID, accountID, currency string, amount int64) error {
	if tx == nil {
		return nil
	}
	systemAccountID, err := h.accounts.GetSystemAccount(ctx, currency)
	if err != nil {
		return err
	}
	transactionID := uuid.NewString()
	metadata, _ := json.Marshal(map[string]string{
		"opening_balance": "true",
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (id, user_id, type, status, amount, currency, from_account_id, to_account_id, metadata)
		VALUES ($1, $2, 'transfer', 'completed', $3, $4, $5, $6, $7)
	`, transactionID, userID, amount, currency, systemAccountID, accountID, string(metadata)); err != nil {
		return err
	}
	entries := []store.LedgerEntryInput{
		{
			ID:            uuid.NewString(),
			TransactionID: transactionID,
			AccountID:     systemAccountID,
			Amount:        -amount,
			Currency:      currency,
			Description:   "Opening balance debit",
		},
		{
			ID:            uuid.NewString(),
			TransactionID: transactionID,
			AccountID:     accountID,
			Amount:        amount,
			Currency:      currency,
			Description:   "Opening balance credit",
		},
	}
	if err := h.ledger.InsertEntries(ctx, tx, entries); err != nil {
		return err
	}
	if _, err := h.accounts.AdjustBalance(ctx, tx, systemAccountID, -amount); err != nil {
		return err
	}
	return nil
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}
	if !auth.CheckPassword(valueToString(user["password_hash"]), req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := h.txRunner.WithTx(r.Context(), func(tx *sqlx.Tx) error {
		data, _ := json.Marshal(map[string]string{
			"user_id":    valueToString(user["id"]),
			"ip":         r.RemoteAddr,
			"user_agent": r.UserAgent(),
		})
		return h.audit.Log(r.Context(), tx, valueToString(user["id"]), "login", "user", valueToString(user["id"]), string(data))
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}
	token, err := auth.GenerateToken(h.cfg.JWTSecret, valueToString(user["id"]), h.cfg.TokenTTL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load user")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"id":         valueToString(user["id"]),
		"username":   valueToString(user["username"]),
		"email":      valueToString(user["email"]),
		"created_at": user["created_at"],
	})
}
