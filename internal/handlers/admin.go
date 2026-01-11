package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"banking/internal/auth"
	"banking/internal/middleware"
	"banking/internal/websocket"

	"github.com/jmoiron/sqlx"
)

type promoteRequest struct {
	Identifier string `json:"identifier"`
}

func (h *Handler) PromoteAdmin(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, isSuper, err := h.admin.IsAdmin(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to verify admin")
		return
	}
	if !isSuper {
		respondError(w, http.StatusForbidden, "super_admin_required")
		return
	}
	var req promoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Identifier == "" {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	var username string
	var email string
	if strings.Contains(req.Identifier, "@") {
		email = req.Identifier
	} else {
		username = req.Identifier
	}
	targetUserID, err := h.resolveUserID(r.Context(), username, email)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "unable to resolve user")
		return
	}
	err = h.txRunner.WithTx(r.Context(), func(tx *sqlx.Tx) error {
		if err := h.admin.CreateAdmin(r.Context(), tx, targetUserID, false, &userID); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]string{
			"target_user_id": targetUserID,
		})
		return h.audit.Log(r.Context(), tx, userID, "promote_admin", "admin", targetUserID, string(data))
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to promote admin")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"status": "promoted"})
}

type grantRoleRequest struct {
	AdminUserID string `json:"admin_user_id"`
	Role        string `json:"role"`
}

func (h *Handler) GrantRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, isSuper, err := h.admin.IsAdmin(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to verify admin")
		return
	}
	if !isSuper {
		respondError(w, http.StatusForbidden, "super_admin_required")
		return
	}
	var req grantRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AdminUserID == "" || req.Role == "" {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	isAdmin, isSuper, err := h.admin.IsAdmin(r.Context(), req.AdminUserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to verify target admin")
		return
	}
	if !isAdmin {
		respondError(w, http.StatusBadRequest, "target is not an admin")
		return
	}
	if isSuper {
		respondError(w, http.StatusBadRequest, "cannot assign roles to super admin")
		return
	}
	err = h.txRunner.WithTx(r.Context(), func(tx *sqlx.Tx) error {
		if err := h.admin.GrantRole(r.Context(), tx, req.AdminUserID, req.Role); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]string{
			"admin_user_id": req.AdminUserID,
			"role":          req.Role,
		})
		return h.audit.Log(r.Context(), tx, userID, "grant_role", "admin_role", req.AdminUserID, string(data))
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to grant role")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"status": "role_granted"})
}

type exchangeRateRequest struct {
	QuoteCurrency string `json:"quote_currency"`
	Rate          string `json:"rate"`
}

func (h *Handler) SetExchangeRate(w http.ResponseWriter, r *http.Request) {
	respondError(w, http.StatusForbidden, "rate_fixed")
}

func (h *Handler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.accounts.ListAllWithUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load users")
		return
	}
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		username := ""
		if row.Username != nil {
			username = *row.Username
		}
		email := ""
		if row.Email != nil {
			email = *row.Email
		}
		normalized = append(normalized, map[string]any{
			"account_id": row.ID,
			"currency":   row.Currency,
			"balance":    valueToMoney(row.Balance),
			"is_system":  row.IsSystem,
			"username":   username,
			"email":      email,
			"created_at": row.CreatedAt,
		})
	}
	respondJSON(w, http.StatusOK, normalized)
}

func (h *Handler) AdminListTransactions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := parseInt(query.Get("limit"), 50)
	page := parseInt(query.Get("page"), 1)
	offset := (page - 1) * limit
	rows, err := h.transactions.ListAll(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load transactions")
		return
	}
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		normalized = append(normalized, map[string]any{
			"id":               valueToString(row["id"]),
			"user_id":          valueToString(row["user_id"]),
			"username":         valueToString(row["username"]),
			"from_username":    valueToString(row["from_username"]),
			"to_username":      valueToString(row["to_username"]),
			"type":             valueToString(row["type"]),
			"status":           valueToString(row["status"]),
			"amount":           valueToMoney(row["amount"]),
			"currency":         valueToString(row["currency"]),
			"from_account_id":  valueToString(row["from_account_id"]),
			"to_account_id":    valueToString(row["to_account_id"]),
			"exchange_rate_id": valueToString(row["exchange_rate_id"]),
			"metadata":         row["metadata"],
			"created_at":       row["created_at"],
		})
	}
	respondJSON(w, http.StatusOK, normalized)
}

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := parseInt(query.Get("limit"), 50)
	page := parseInt(query.Get("page"), 1)
	offset := (page - 1) * limit
	rows, err := h.audit.List(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load audit logs")
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	type reconRow struct {
		AccountID      string `db:"account_id"`
		LedgerSum      int64  `db:"ledger_sum"`
		AccountBalance int64  `db:"account_balance"`
		Difference     int64  `db:"difference"`
	}
	var rows []reconRow
	query := `
		SELECT a.id AS account_id,
		       COALESCE(SUM(l.amount), 0) AS ledger_sum,
		       a.balance AS account_balance,
		       (a.balance - COALESCE(SUM(l.amount), 0)) AS difference
		FROM accounts a
		LEFT JOIN ledger_entries l ON l.account_id = a.id
		GROUP BY a.id, a.balance
		ORDER BY a.id
	`
	if err := h.reconcileDB.SelectContext(r.Context(), &rows, query); err != nil {
		respondError(w, http.StatusInternalServerError, "unable to reconcile balances")
		return
	}
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		normalized = append(normalized, map[string]any{
			"account_id":      row.AccountID,
			"ledger_sum":      valueToMoney(row.LedgerSum),
			"account_balance": valueToMoney(row.AccountBalance),
			"difference":      valueToMoney(row.Difference),
		})
	}
	respondJSON(w, http.StatusOK, normalized)
}

func (h *Handler) WSBalances(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		respondError(w, http.StatusUnauthorized, "missing token")
		return
	}
	claims, err := auth.ParseToken(h.cfg.JWTSecret, token)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	websocket.ServeWS(w, r, h.hub, claims.UserID)
}
