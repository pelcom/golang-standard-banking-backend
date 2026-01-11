package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
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

func (h *Handler) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
	emailParam := chi.URLParam(r, "email")
	email, err := url.PathUnescape(emailParam)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid email")
		return
	}
	user, err := h.users.GetByEmail(r.Context(), email)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("user err", err)
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
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
