package services

import (
	"testing"

	"banking/internal/store"
)

func TestEnsureBalanced(t *testing.T) {
	entries := []store.LedgerEntryInput{
		{Amount: 1000, Currency: "USD"},
		{Amount: -1000, Currency: "USD"},
	}
	if err := ensureBalanced(entries); err != nil {
		t.Fatalf("expected balanced entries, got error: %v", err)
	}
	entries = append(entries, store.LedgerEntryInput{Amount: 100, Currency: "USD"})
	if err := ensureBalanced(entries); err == nil {
		t.Fatal("expected imbalance error")
	}
}

func TestEnsureBalancedByCurrency(t *testing.T) {
	entries := []store.LedgerEntryInput{
		{Amount: 1000, Currency: "USD"},
		{Amount: -1000, Currency: "USD"},
		{Amount: 500, Currency: "EUR"},
		{Amount: -500, Currency: "EUR"},
	}
	if err := ensureBalancedByCurrency(entries); err != nil {
		t.Fatalf("expected balanced entries, got error: %v", err)
	}
	entries = append(entries, store.LedgerEntryInput{Amount: 1, Currency: "EUR"})
	if err := ensureBalancedByCurrency(entries); err == nil {
		t.Fatal("expected imbalance error")
	}
}
