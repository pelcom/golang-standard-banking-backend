package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"banking/internal/money"
)

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func valueToString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func valueToMoney(value any) string {
	return money.FormatMinor(money.ValueToInt64(value))
}
