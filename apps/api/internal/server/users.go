package server

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type userPayload struct {
	Email    Opt[string]    `json:"email"`
	Name     Opt[string]    `json:"name"`
	Role     Opt[core.Role] `json:"role"`
	IsActive Opt[bool]      `json:"is_active"`
	Password Opt[string]    `json:"password"`
}

func (p userPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "email", p.Email.Set, p.Email.Null)
	nullOn(fields, "name", p.Name.Set, p.Name.Null)
	nullOn(fields, "role", p.Role.Set, p.Role.Null)
	nullOn(fields, "is_active", p.IsActive.Set, p.IsActive.Null)
	nullOn(fields, "password", p.Password.Set, p.Password.Null)
	return fieldError(fields)
}

func (p userPayload) applyTo(u *core.User) {
	p.Email.apply(&u.Email)
	p.Name.apply(&u.Name)
	p.Role.apply(&u.Role)
	p.IsActive.apply(&u.IsActive)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var payload userPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	user := core.User{IsActive: true}
	payload.applyTo(&user)
	if err := s.store.CreateUser(&user, payload.Password.Value); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	user, err := s.store.UserByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	user, err := s.store.UserByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload userPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(user)
	if err := s.store.UpdateUser(currentUser(r).ID, user); err != nil {
		writeError(w, err)
		return
	}
	if payload.Password.Set && !payload.Password.Null {
		if err := s.store.SetUserPassword(user.ID, payload.Password.Value); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var filter store.UserFilter
	if raw := r.URL.Query().Get("role"); raw != "" {
		role, ok := core.ParseRole(raw)
		if !ok {
			writeError(w, core.Invalid("unknown role"))
			return
		}
		filter.Role = &role
	}
	if filter.Active, err = queryBool(r, "is_active"); err != nil {
		writeError(w, err)
		return
	}
	items, total, err := s.store.ListUsers(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
