// Package handlers wires HTTP endpoints to repositories and the auth package.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Users wires user signup/login/me.
type Users struct {
	Repo   postgres.UserRepo
	Issuer *auth.Issuer
}

type signupRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token     string      `json:"token"`
	ExpiresAt int64       `json:"expires_at"`
	User      publicUser  `json:"user"`
}

type publicUser struct {
	ID    string      `json:"id"`
	Email string      `json:"email"`
	Name  string      `json:"name,omitempty"`
	Roles []auth.Role `json:"roles"`
}

// Signup creates a new user (default role: "user") and returns a JWT.
func (h *Users) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		http.Error(w, "email and 8+ char password required", http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "hash failed", http.StatusInternalServerError)
		return
	}
	user, err := h.Repo.Create(r.Context(), req.Email, req.Name, hash, []auth.Role{auth.RoleUser})
	if err != nil {
		http.Error(w, "could not create user: "+err.Error(), http.StatusConflict)
		return
	}
	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		http.Error(w, "could not sign token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, tokenResponse{
		Token: tok, ExpiresAt: claims.ExpiresAt,
		User: publicUser{ID: user.ID, Email: user.Email, Name: user.Name, Roles: user.Roles},
	})
}

// Login validates credentials and returns a fresh JWT.
func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	user, err := h.Repo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, postgres.ErrUserNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		http.Error(w, "could not sign token", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, tokenResponse{
		Token: tok, ExpiresAt: claims.ExpiresAt,
		User: publicUser{ID: user.ID, Email: user.Email, Name: user.Name, Roles: user.Roles},
	})
}

// Me returns the authenticated user's public profile.
func (h *Users) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	user, err := h.Repo.GetByID(r.Context(), claims.Subject)
	if err != nil {
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, publicUser{ID: user.ID, Email: user.Email, Name: user.Name, Roles: user.Roles})
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
