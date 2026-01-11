package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrTooManyDecimals  = errors.New("amount has too many decimal places")
)

func ParseMinor(input string) (int64, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, ErrInvalidAmount
	}
	sign := int64(1)
	switch trimmed[0] {
	case '-':
		sign = -1
		trimmed = trimmed[1:]
	case '+':
		trimmed = trimmed[1:]
	}
	parts := strings.SplitN(trimmed, ".", 2)
	wholePart := parts[0]
	if wholePart == "" {
		wholePart = "0"
	}
	if !isDigits(wholePart) {
		return 0, ErrInvalidAmount
	}
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if len(fracPart) > 2 {
		return 0, ErrTooManyDecimals
	}
	if fracPart != "" && !isDigits(fracPart) {
		return 0, ErrInvalidAmount
	}
	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	frac := int64(0)
	if len(fracPart) == 1 {
		frac = int64(fracPart[0]-'0') * 10
	} else if len(fracPart) == 2 {
		value, err := strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
		frac = value
	}
	minor := whole*100 + frac
	return sign * minor, nil
}

func FormatMinor(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	whole := value / 100
	frac := value % 100
	formatted := fmt.Sprintf("%d.%02d", whole, frac)
	if negative {
		return "-" + formatted
	}
	return formatted
}

func ValueToInt64(value interface{}) int64 {
	switch v := value.(type) {
	case nil:
		return 0
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case []byte:
		parsed, _ := strconv.ParseInt(string(v), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return parsed
	}
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
