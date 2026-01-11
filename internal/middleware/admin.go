package middleware

import (
	"context"
	"net/http"
)

type AdminStore interface {
	IsAdmin(ctx context.Context, userID string) (bool, bool, error)
	HasRole(ctx context.Context, userID, role string) (bool, error)
}

func RequireAdmin(adminStore AdminStore, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			isAdmin, isSuper, err := adminStore.IsAdmin(r.Context(), userID)
			if err != nil {
				http.Error(w, "unable to verify admin", http.StatusInternalServerError)
				return
			}
			if !isAdmin {
				http.Error(w, "admin privileges required", http.StatusForbidden)
				return
			}
			if isSuper {
				next.ServeHTTP(w, r)
				return
			}
			if role == "" {
				next.ServeHTTP(w, r)
				return
			}
			hasRole, err := adminStore.HasRole(r.Context(), userID, role)
			if err != nil {
				http.Error(w, "unable to verify role", http.StatusInternalServerError)
				return
			}
			if !hasRole {
				http.Error(w, "missing required role", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
