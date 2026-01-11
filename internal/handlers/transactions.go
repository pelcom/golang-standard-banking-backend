package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"banking/internal/middleware"
	"banking/internal/money"
	"banking/internal/services"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type transferRequest struct {
	FromAccountID   string  `json:"from_account_id"`
	ToAccountID     string  `json:"to_account_id"`
	ToUsername      string  `json:"to_username"`
	ToEmail         string  `json:"to_email"`
	Amount          string  `json:"amount"`
	Confirm         bool    `json:"confirm"`
	ClientRequestID *string `json:"client_request_id"`
}

func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	log.Println("userID userID", userID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if !req.Confirm {
		respondError(w, http.StatusBadRequest, "confirmation_required")
		return
	}
	if req.FromAccountID == "" {
		respondError(w, http.StatusBadRequest, "from_account_id is required")
		return
	}
	amountMinor, err := parseAmountMinor(req.Amount)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_amount")
		return
	}
	toAccountID := strings.TrimSpace(req.ToAccountID)
	if toAccountID == "" {
		targetUserID, err := h.resolveUserID(r.Context(), req.ToUsername, req.ToEmail)
		if err != nil {
			if err == sql.ErrNoRows {
				respondError(w, http.StatusNotFound, "recipient not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "unable to resolve recipient")
			return
		}
		fromAccount, err := h.accounts.GetByID(r.Context(), req.FromAccountID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid from account")
			return
		}
		targetAccount, err := h.accounts.GetByUserAndCurrency(r.Context(), targetUserID, fromAccount.Currency)
		log.Println("targetAccount targetAccount", targetAccount)
		if err != nil {
			respondError(w, http.StatusNotFound, "recipient account not found")
			return
		}
		toAccountID = targetAccount.ID
	}
	transactionID, err := h.service.Transfer(r.Context(), services.TransferRequest{
		UserID:          userID,
		FromAccountID:   req.FromAccountID,
		ToAccountID:     toAccountID,
		AmountMinor:     amountMinor,
		ClientRequestID: req.ClientRequestID,
	})
	log.Println("transactionID err", err)
	if err != nil {
		if err == services.ErrInsufficientFunds {
			respondError(w, http.StatusBadRequest, "insufficient_funds")
			return
		}
		if err == services.ErrCurrencyMismatch {
			respondError(w, http.StatusBadRequest, "currency_mismatch")
			return
		}
		if err == services.ErrUnauthorizedAccount {
			respondError(w, http.StatusForbidden, "account_access_denied")
			return
		}
		if err == services.ErrInvalidAmount {
			respondError(w, http.StatusBadRequest, "invalid_amount")
			return
		}
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			respondError(w, http.StatusConflict, "duplicate_request")
			return
		}
		respondError(w, http.StatusInternalServerError, "transfer_failed")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"transaction_id": transactionID})
}

type exchangeRequest struct {
	FromAccountID   string  `json:"from_account_id"`
	ToAccountID     string  `json:"to_account_id"`
	Amount          string  `json:"amount"`
	Confirm         bool    `json:"confirm"`
	ClientRequestID *string `json:"client_request_id"`
	QuoteID         *string `json:"quote_id"`
	QuotedRate      *string `json:"quoted_rate"`
}

type exchangeQuoteRequest struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        string `json:"amount"`
}

func (h *Handler) ExchangeQuote(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req exchangeQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	amountMinor, err := parseAmountMinor(req.Amount)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_amount")
		return
	}
	quote, err := h.service.QuoteExchange(r.Context(), services.ExchangeQuoteRequest{
		UserID:        userID,
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		AmountMinor:   amountMinor,
	})
	if err != nil {
		switch err {
		case services.ErrExchangeRateNotSet:
			respondError(w, http.StatusBadRequest, "exchange_rate_not_set")
		case services.ErrUnauthorizedAccount:
			respondError(w, http.StatusForbidden, "account_access_denied")
		case services.ErrInvalidAmount:
			respondError(w, http.StatusBadRequest, "invalid_amount")
		default:
			respondError(w, http.StatusBadRequest, "invalid_exchange_request")
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"quote_id":         quote.ID,
		"rate":             quote.Rate,
		"converted_amount": valueToMoney(quote.ConvertedMinor),
		"expires_at":       quote.ExpiresAt,
	})
}

func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req exchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if !req.Confirm {
		respondError(w, http.StatusBadRequest, "confirmation_required")
		return
	}
	amountMinor, err := parseAmountMinor(req.Amount)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_amount")
		return
	}
	var quotedRate *string
	if req.QuoteID == nil || *req.QuoteID == "" {
		if req.QuotedRate == nil || *req.QuotedRate == "" {
			respondError(w, http.StatusBadRequest, "quote_required")
			return
		}
		rate, err := parseRate(*req.QuotedRate)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_rate")
			return
		}
		normalized := rate.StringFixedBank(6)
		quotedRate = &normalized
	}
	transactionID, err := h.service.Exchange(r.Context(), services.ExchangeRequest{
		UserID:          userID,
		FromAccountID:   req.FromAccountID,
		ToAccountID:     req.ToAccountID,
		AmountMinor:     amountMinor,
		ClientRequestID: req.ClientRequestID,
		QuoteID:         req.QuoteID,
		QuotedRate:      quotedRate,
	})
	if err != nil {
		switch err {
		case services.ErrExchangeRateNotSet:
			respondError(w, http.StatusBadRequest, "exchange_rate_not_set")
		case services.ErrInsufficientFunds:
			respondError(w, http.StatusBadRequest, "insufficient_funds")
		case services.ErrInvalidAmount:
			respondError(w, http.StatusBadRequest, "invalid_amount")
		case services.ErrUnauthorizedAccount:
			respondError(w, http.StatusForbidden, "account_access_denied")
		case services.ErrInvalidExchangeRequest:
			respondError(w, http.StatusBadRequest, "invalid_exchange_request")
		case services.ErrQuoteExpired:
			respondError(w, http.StatusBadRequest, "quote_expired")
		case services.ErrQuoteNotFound:
			respondError(w, http.StatusBadRequest, "quote_not_found")
		case services.ErrQuoteConsumed:
			respondError(w, http.StatusBadRequest, "quote_consumed")
		case services.ErrRateMismatch:
			respondError(w, http.StatusBadRequest, "quoted_rate_mismatch")
		default:
			if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
				respondError(w, http.StatusConflict, "duplicate_request")
				return
			}
			respondError(w, http.StatusInternalServerError, "exchange_failed")
		}
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"transaction_id": transactionID})
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	query := r.URL.Query()
	txType := query.Get("type")
	page := parseInt(query.Get("page"), 1)
	limit := parseInt(query.Get("limit"), 20)
	offset := (page - 1) * limit
	transactions, err := h.transactions.ListByUser(r.Context(), userID, txType, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "unable to load transactions")
		return
	}
	normalized := make([]map[string]any, 0, len(transactions))
	for _, row := range transactions {
		metadata := parseMetadata(row["metadata"])
		rate, convertedAmount := exchangeDetails(row, metadata)
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
			"rate":             rate,
			"converted_amount": convertedAmount,
			"from_currency":    valueToString(row["currency"]),
			"to_currency":      otherCurrency(valueToString(row["currency"])),
			"metadata":         row["metadata"],
			"created_at":       row["created_at"],
		})
	}
	respondJSON(w, http.StatusOK, normalized)
}

func parseInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (h *Handler) resolveUserID(ctx context.Context, username, email string) (string, error) {
	if username != "" {
		user, err := h.users.GetByUsername(ctx, username)
		if err != nil {
			return "", err
		}
		return valueToString(user["id"]), nil
	}
	if email != "" {
		user, err := h.users.GetByEmail(ctx, email)
		if err != nil {
			return "", err
		}
		return valueToString(user["id"]), nil
	}
	return "", sql.ErrNoRows
}

func parseMetadata(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case []byte:
		var parsed map[string]any
		_ = json.Unmarshal(v, &parsed)
		return parsed
	case string:
		var parsed map[string]any
		_ = json.Unmarshal([]byte(v), &parsed)
		return parsed
	default:
		return nil
	}
}

func exchangeDetails(row map[string]any, metadata map[string]any) (string, string) {
	if valueToString(row["type"]) != "exchange" || metadata == nil {
		return "", ""
	}
	rateRaw := valueToString(metadata["rate"])
	if rateRaw == "" {
		return "", ""
	}
	rate, err := decimal.NewFromString(rateRaw)
	if err != nil {
		return "", ""
	}
	amountMinor := money.ValueToInt64(row["amount"])
	convertedMinor := decimal.NewFromInt(amountMinor).Mul(rate).RoundBank(0).IntPart()
	return rate.StringFixedBank(6), money.FormatMinor(convertedMinor)
}

func otherCurrency(currency string) string {
	switch currency {
	case "USD":
		return "EUR"
	case "EUR":
		return "USD"
	default:
		return ""
	}
}
