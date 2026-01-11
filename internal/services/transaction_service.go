package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"banking/internal/db"
	"banking/internal/money"
	"banking/internal/store"
	"banking/internal/websocket"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

var (
	ErrInsufficientFunds      = errors.New("insufficient funds")
	ErrCurrencyMismatch       = errors.New("currency mismatch")
	ErrExchangeRateNotSet     = errors.New("exchange rate not set")
	ErrInvalidAmount          = errors.New("invalid amount")
	ErrSameAccountTransfer    = errors.New("cannot transfer to same account")
	ErrUnauthorizedAccount    = errors.New("account does not belong to user")
	ErrInvalidExchangeRequest = errors.New("invalid exchange request")
	ErrQuoteNotFound          = errors.New("quote not found")
	ErrQuoteExpired           = errors.New("quote expired")
	ErrQuoteConsumed          = errors.New("quote already used")
	ErrRateMismatch           = errors.New("quoted rate mismatch")
)

type TransactionService struct {
	txRunner      db.TxRunner
	accountStore  AccountStore
	ledgerStore   LedgerStore
	txStore       TransactionStore
	exchangeStore ExchangeStore
	quoteStore    ExchangeQuoteStore
	auditStore    AuditStore
	hub           BalanceHub
}

type AccountStore interface {
	GetByID(ctx context.Context, accountID string) (store.Account, error)
	GetForUpdate(ctx context.Context, tx store.Getter, accountID string) (store.Account, error)
	UpdateBalance(ctx context.Context, tx store.Execer, accountID string, balance int64) error
	GetSystemAccount(ctx context.Context, currency string) (string, error)
}

type LedgerStore interface {
	InsertEntries(ctx context.Context, tx store.Execer, entries []store.LedgerEntryInput) error
}

type TransactionStore interface {
	Create(ctx context.Context, tx store.Execer, input store.TransactionInput) error
}

type ExchangeStore interface {
	GetActive(ctx context.Context, baseCurrency, quoteCurrency string) (map[string]any, error)
}

type ExchangeQuoteStore interface {
	Create(ctx context.Context, input store.ExchangeQuoteInput) error
	GetByID(ctx context.Context, quoteID string) (store.ExchangeQuote, error)
	Consume(ctx context.Context, tx store.Execer, quoteID string) (int64, error)
}

type AuditStore interface {
	Log(ctx context.Context, tx store.Execer, actorID, action, entityType, entityID, data string) error
}

type BalanceHub interface {
	BroadcastBalance(userID string, update websocket.BalanceUpdate)
}

func NewTransactionService(txRunner db.TxRunner, accountStore AccountStore, ledgerStore LedgerStore, txStore TransactionStore, exchangeStore ExchangeStore, quoteStore ExchangeQuoteStore, auditStore AuditStore, hub BalanceHub) *TransactionService {
	return &TransactionService{
		txRunner:      txRunner,
		accountStore:  accountStore,
		ledgerStore:   ledgerStore,
		txStore:       txStore,
		exchangeStore: exchangeStore,
		quoteStore:    quoteStore,
		auditStore:    auditStore,
		hub:           hub,
	}
}

type TransferRequest struct {
	UserID          string
	FromAccountID   string
	ToAccountID     string
	AmountMinor     int64
	ClientRequestID *string
}

func (s *TransactionService) Transfer(ctx context.Context, req TransferRequest) (string, error) {
	if req.AmountMinor <= 0 {
		return "", ErrInvalidAmount
	}
	if req.FromAccountID == req.ToAccountID {
		return "", ErrSameAccountTransfer
	}
	var transactionID string
	var toUserID string
	var fromBalanceAfter int64
	var toBalanceAfter int64
	var currency string
	err := s.txRunner.WithTx(ctx, func(tx *sqlx.Tx) error {
		fromAccount, toAccount, err := lockTwoAccounts(ctx, tx, s.accountStore, req.FromAccountID, req.ToAccountID)
		if err != nil {
			return err
		}
		log.Println("fromAccount", fromAccount)
		log.Println("fromAccount user_id", fromAccount.UserID)
		log.Println("req.UserID", req.UserID)
		if fromAccount.UserID == nil || *fromAccount.UserID != req.UserID {
			return ErrUnauthorizedAccount
		}
		if fromAccount.Currency != toAccount.Currency {
			return ErrCurrencyMismatch
		}
		currency = fromAccount.Currency
		if toAccount.UserID != nil {
			toUserID = *toAccount.UserID
		}
		fromBalance := fromAccount.Balance
		if fromBalance < req.AmountMinor {
			return ErrInsufficientFunds
		}
		newFrom := fromBalance - req.AmountMinor
		toBalance := toAccount.Balance
		newTo := toBalance + req.AmountMinor
		fromBalanceAfter = newFrom
		toBalanceAfter = newTo
		if err := s.accountStore.UpdateBalance(ctx, tx, req.FromAccountID, newFrom); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, req.ToAccountID, newTo); err != nil {
			return err
		}

		transactionID = uuid.NewString()
		if err := s.txStore.Create(ctx, tx, store.TransactionInput{
			ID:              transactionID,
			UserID:          req.UserID,
			Type:            "transfer",
			Status:          "completed",
			Amount:          req.AmountMinor,
			Currency:        currency,
			FromAccountID:   &req.FromAccountID,
			ToAccountID:     &req.ToAccountID,
			Metadata:        "{}",
			ClientRequestID: req.ClientRequestID,
		}); err != nil {
			return err
		}
		entries := []store.LedgerEntryInput{
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     req.FromAccountID,
				Amount:        -req.AmountMinor,
				Currency:      currency,
				Description:   "Transfer debit",
			},
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     req.ToAccountID,
				Amount:        req.AmountMinor,
				Currency:      currency,
				Description:   "Transfer credit",
			},
		}
		if err := ensureBalanced(entries); err != nil {
			return err
		}
		if err := s.ledgerStore.InsertEntries(ctx, tx, entries); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]string{
			"transaction_id": transactionID,
		})
		return s.auditStore.Log(ctx, tx, req.UserID, "transfer", "transaction", transactionID, string(data))
	})
	if err != nil {
		return "", err
	}
	s.hub.BroadcastBalance(req.UserID, websocket.BalanceUpdate{
		AccountID: req.FromAccountID,
		Balance:   money.FormatMinor(fromBalanceAfter),
		Currency:  currency,
	})
	if toUserID != "" {
		s.hub.BroadcastBalance(toUserID, websocket.BalanceUpdate{
			AccountID: req.ToAccountID,
			Balance:   money.FormatMinor(toBalanceAfter),
			Currency:  currency,
		})
	}
	return transactionID, nil
}

type ExchangeQuoteRequest struct {
	UserID        string
	FromAccountID string
	ToAccountID   string
	AmountMinor   int64
}

type ExchangeQuote struct {
	ID             string
	Rate           string
	ConvertedMinor int64
	ExpiresAt      time.Time
}

func (s *TransactionService) QuoteExchange(ctx context.Context, req ExchangeQuoteRequest) (ExchangeQuote, error) {
	if req.AmountMinor <= 0 {
		return ExchangeQuote{}, ErrInvalidAmount
	}
	if req.FromAccountID == req.ToAccountID {
		return ExchangeQuote{}, ErrInvalidExchangeRequest
	}
	fromAccount, err := s.accountStore.GetByID(ctx, req.FromAccountID)
	if err != nil {
		return ExchangeQuote{}, err
	}
	toAccount, err := s.accountStore.GetByID(ctx, req.ToAccountID)
	if err != nil {
		return ExchangeQuote{}, err
	}
	if fromAccount.UserID == nil || *fromAccount.UserID != req.UserID {
		return ExchangeQuote{}, ErrUnauthorizedAccount
	}
	fromCurrency := fromAccount.Currency
	toCurrency := toAccount.Currency
	if fromCurrency == toCurrency || !isExchangePairAllowed(fromCurrency, toCurrency) {
		return ExchangeQuote{}, ErrInvalidExchangeRequest
	}
	baseRate := fixedUSDEURRate()
	rate := baseRate
	if fromCurrency == "EUR" && toCurrency == "USD" {
		rate = decimal.NewFromInt(1).Div(baseRate).RoundBank(6)
	}
	convertedMinor := convertMinor(req.AmountMinor, rate)
	quoteID := uuid.NewString()
	expiresAt := time.Now().Add(2 * time.Minute)
	if err := s.quoteStore.Create(ctx, store.ExchangeQuoteInput{
		ID:             quoteID,
		UserID:         req.UserID,
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		AmountMinor:    req.AmountMinor,
		ConvertedMinor: convertedMinor,
		Rate:           rate.StringFixedBank(6),
		BaseCurrency:   fromCurrency,
		QuoteCurrency:  toCurrency,
		ExpiresAt:      expiresAt.UTC(),
	}); err != nil {
		return ExchangeQuote{}, err
	}
	return ExchangeQuote{
		ID:             quoteID,
		Rate:           rate.StringFixedBank(6),
		ConvertedMinor: convertedMinor,
		ExpiresAt:      expiresAt.UTC(),
	}, nil
}

type ExchangeRequest struct {
	UserID          string
	FromAccountID   string
	ToAccountID     string
	AmountMinor     int64
	ClientRequestID *string
	QuoteID         *string
	QuotedRate      *string
}

func (s *TransactionService) Exchange(ctx context.Context, req ExchangeRequest) (string, error) {
	if req.AmountMinor <= 0 {
		return "", ErrInvalidAmount
	}
	var transactionID string
	var fromBalanceAfter int64
	var toBalanceAfter int64
	var toCurrency string
	var fromCurrency string
	var rate decimal.Decimal
	var quoteID string
	var expectedConverted int64

	if req.QuoteID != nil {
		quoteID = *req.QuoteID
		quote, err := s.quoteStore.GetByID(ctx, quoteID)
		if err != nil {
			return "", ErrQuoteNotFound
		}
		if quote.ConsumedAt != nil {
			return "", ErrQuoteConsumed
		}
		if time.Now().After(quote.ExpiresAt) {
			return "", ErrQuoteExpired
		}
		if quote.UserID != req.UserID || quote.FromAccountID != req.FromAccountID || quote.ToAccountID != req.ToAccountID {
			return "", ErrInvalidExchangeRequest
		}
		if quote.AmountMinor != req.AmountMinor {
			return "", ErrInvalidExchangeRequest
		}
		expectedConverted = quote.ConvertedMinor
		rate, err = decimal.NewFromString(quote.Rate)
		if err != nil {
			return "", ErrInvalidExchangeRequest
		}
		fromCurrency = quote.BaseCurrency
		toCurrency = quote.QuoteCurrency
	} else if req.QuotedRate != nil {
		parsed, err := decimal.NewFromString(*req.QuotedRate)
		if err != nil {
			return "", ErrInvalidExchangeRequest
		}
		rate = parsed
	} else {
		return "", ErrInvalidExchangeRequest
	}

	err := s.txRunner.WithTx(ctx, func(tx *sqlx.Tx) error {
		fromAccount, toAccount, err := lockTwoAccounts(ctx, tx, s.accountStore, req.FromAccountID, req.ToAccountID)
		if err != nil {
			return err
		}
		if fromAccount.UserID == nil || *fromAccount.UserID != req.UserID {
			return ErrUnauthorizedAccount
		}
		fromCurrency = fromAccount.Currency
		toCurrency = toAccount.Currency
		if fromCurrency == toCurrency || !isExchangePairAllowed(fromCurrency, toCurrency) {
			return ErrInvalidExchangeRequest
		}
		baseRate := fixedUSDEURRate()
		directionalRate := baseRate
		if fromCurrency == "EUR" && toCurrency == "USD" {
			directionalRate = decimal.NewFromInt(1).Div(baseRate).RoundBank(6)
		}
		if !rate.Equal(directionalRate) {
			return ErrRateMismatch
		}
		convertedMinor := convertMinor(req.AmountMinor, directionalRate)
		if expectedConverted != 0 && expectedConverted != convertedMinor {
			return ErrRateMismatch
		}

		fromBalance := fromAccount.Balance
		if fromBalance < req.AmountMinor {
			return ErrInsufficientFunds
		}
		toBalance := toAccount.Balance
		newFrom := fromBalance - req.AmountMinor
		newTo := toBalance + convertedMinor
		fromBalanceAfter = newFrom
		toBalanceAfter = newTo

		systemFromID, err := s.accountStore.GetSystemAccount(ctx, fromCurrency)
		if err != nil {
			return err
		}
		systemToID, err := s.accountStore.GetSystemAccount(ctx, toCurrency)
		if err != nil {
			return err
		}
		systemFrom, systemTo, err := lockTwoAccounts(ctx, tx, s.accountStore, systemFromID, systemToID)
		if err != nil {
			return err
		}
		systemFromBalance := systemFrom.Balance
		systemToBalance := systemTo.Balance
		newSystemFrom := systemFromBalance + req.AmountMinor
		newSystemTo := systemToBalance - convertedMinor
		if newSystemTo < 0 {
			return ErrInsufficientFunds
		}

		if err := s.accountStore.UpdateBalance(ctx, tx, req.FromAccountID, newFrom); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, req.ToAccountID, newTo); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, systemFromID, newSystemFrom); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, systemToID, newSystemTo); err != nil {
			return err
		}

		transactionID = uuid.NewString()
		metadata, _ := json.Marshal(map[string]string{
			"rate":     directionalRate.StringFixedBank(6),
			"quote_id": quoteID,
		})
		if err := s.txStore.Create(ctx, tx, store.TransactionInput{
			ID:              transactionID,
			UserID:          req.UserID,
			Type:            "exchange",
			Status:          "completed",
			Amount:          req.AmountMinor,
			Currency:        fromCurrency,
			FromAccountID:   &req.FromAccountID,
			ToAccountID:     &req.ToAccountID,
			ExchangeRateID:  nil,
			Metadata:        string(metadata),
			ClientRequestID: req.ClientRequestID,
		}); err != nil {
			return err
		}
		entries := []store.LedgerEntryInput{
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     req.FromAccountID,
				Amount:        -req.AmountMinor,
				Currency:      fromCurrency,
				Description:   "Exchange debit",
			},
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     systemFromID,
				Amount:        req.AmountMinor,
				Currency:      fromCurrency,
				Description:   "Exchange system credit",
			},
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     systemToID,
				Amount:        -convertedMinor,
				Currency:      toCurrency,
				Description:   "Exchange system debit",
			},
			{
				ID:            uuid.NewString(),
				TransactionID: transactionID,
				AccountID:     req.ToAccountID,
				Amount:        convertedMinor,
				Currency:      toCurrency,
				Description:   "Exchange credit",
			},
		}
		if err := ensureBalancedByCurrency(entries); err != nil {
			return err
		}
		if err := s.ledgerStore.InsertEntries(ctx, tx, entries); err != nil {
			return err
		}
		if err := s.auditStore.Log(ctx, tx, req.UserID, "exchange", "transaction", transactionID, string(metadata)); err != nil {
			return err
		}
		if quoteID != "" {
			consumed, err := s.quoteStore.Consume(ctx, tx, quoteID)
			if err != nil {
				return err
			}
			if consumed == 0 {
				return ErrQuoteConsumed
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	s.hub.BroadcastBalance(req.UserID, websocket.BalanceUpdate{
		AccountID: req.FromAccountID,
		Balance:   money.FormatMinor(fromBalanceAfter),
		Currency:  fromCurrency,
	})
	s.hub.BroadcastBalance(req.UserID, websocket.BalanceUpdate{
		AccountID: req.ToAccountID,
		Balance:   money.FormatMinor(toBalanceAfter),
		Currency:  toCurrency,
	})
	return transactionID, nil
}

func ensureBalanced(entries []store.LedgerEntryInput) error {
	var sum int64
	for _, entry := range entries {
		sum += entry.Amount
	}
	if sum != 0 {
		return errors.New("ledger entries are not balanced")
	}
	return nil
}

func ensureBalancedByCurrency(entries []store.LedgerEntryInput) error {
	sums := map[string]int64{}
	for _, entry := range entries {
		sums[entry.Currency] += entry.Amount
	}
	for _, sum := range sums {
		if sum != 0 {
			return errors.New("ledger entries are not balanced per currency")
		}
	}
	return nil
}

func convertMinor(amountMinor int64, rate decimal.Decimal) int64 {
	return decimal.NewFromInt(amountMinor).Mul(rate).RoundBank(0).IntPart()
}

func fixedUSDEURRate() decimal.Decimal {
	rate, err := decimal.NewFromString("0.92")
	if err != nil {
		return decimal.NewFromFloat(0.92)
	}
	return rate
}

func isExchangePairAllowed(fromCurrency, toCurrency string) bool {
	return (fromCurrency == "USD" && toCurrency == "EUR") || (fromCurrency == "EUR" && toCurrency == "USD")
}

func stringPtr(value string) *string {
	return &value
}

func lockTwoAccounts(ctx context.Context, tx store.Getter, accountStore AccountStore, firstID, secondID string) (store.Account, store.Account, error) {
	leftID, rightID := orderedIDs(firstID, secondID)
	leftAccount, err := accountStore.GetForUpdate(ctx, tx, leftID)
	if err != nil {
		return store.Account{}, store.Account{}, err
	}
	rightAccount, err := accountStore.GetForUpdate(ctx, tx, rightID)
	if err != nil {
		return store.Account{}, store.Account{}, err
	}
	if firstID == leftID {
		return leftAccount, rightAccount, nil
	}
	return rightAccount, leftAccount, nil
}

func orderedIDs(firstID, secondID string) (string, string) {
	if firstID <= secondID {
		return firstID, secondID
	}
	return secondID, firstID
}

func valueToString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}
