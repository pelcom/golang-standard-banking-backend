package handlers

import (
	"net/http"

	"banking/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	accounts, err := h.accounts.GetByUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load accounts")
		return
	}
	normalized := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		accountUserID := ""
		if account.UserID != nil {
			accountUserID = *account.UserID
		}
		normalized = append(normalized, map[string]any{
			"id":             account.ID,
			"user_id":        accountUserID,
			"currency":       account.Currency,
			"balance":        valueToMoney(account.CalculatedBalance),
			"stored_balance": valueToMoney(account.StoredBalance),
			"difference":     valueToMoney(account.Difference),
			"is_system":      account.IsSystem,
			"created_at":     account.CreatedAt,
		})
	}
	respondJSON(w, http.StatusOK, normalized)
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	accountID := chi.URLParam(r, "id")
	account, err := h.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusNotFound, "account not found")
		return
	}
	if account.UserID == nil || *account.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"account_id": accountID,
		"balance":    valueToMoney(account.Balance),
		"currency":   account.Currency,
	})
}

func (h *Handler) SelfCheck(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	type row struct {
		AccountID      string `db:"account_id"`
		Currency       string `db:"currency"`
		AccountBalance int64  `db:"account_balance"`
		LedgerSum      int64  `db:"ledger_sum"`
		Difference     int64  `db:"difference"`
	}
	query := `
		SELECT a.id AS account_id,
		       a.currency,
		       a.balance AS account_balance,
		       COALESCE(SUM(l.amount), 0) AS ledger_sum,
		       (a.balance - COALESCE(SUM(l.amount), 0)) AS difference
		FROM accounts a
		LEFT JOIN ledger_entries l ON l.account_id = a.id
		WHERE a.user_id = $1
		GROUP BY a.id, a.currency, a.balance
		ORDER BY a.currency
	`
	var rows []row
	if err := h.reconcileDB.SelectContext(r.Context(), &rows, query, userID); err != nil {
		respondError(w, http.StatusInternalServerError, "unable to self_check")
		return
	}
	response := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		response = append(response, map[string]any{
			"account_id":      item.AccountID,
			"currency":        item.Currency,
			"account_balance": valueToMoney(item.AccountBalance),
			"ledger_sum":      valueToMoney(item.LedgerSum),
			"difference":      valueToMoney(item.Difference),
		})
	}
	respondJSON(w, http.StatusOK, response)
}
