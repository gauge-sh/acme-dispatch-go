package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

type contextKey struct{}

var userContextKey contextKey

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}
		user, err := s.store.UserForToken(token)
		if err != nil {
			writeError(w, err)
			return
		}
		if user == nil {
			writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func currentUser(r *http.Request) *core.User {
	user, _ := r.Context().Value(userContextKey).(*core.User)
	return user
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user := currentUser(r); user == nil || user.Role != core.RoleAdmin {
			writeErrorCode(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}
		next(w, r)
	}
}
