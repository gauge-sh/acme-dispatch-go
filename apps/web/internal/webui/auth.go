package webui

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

const sessionName = "dispatch"

type contextKey struct{}

var userContextKey contextKey

func currentUser(r *http.Request) *core.User {
	user, _ := r.Context().Value(userContextKey).(*core.User)
	return user
}

// auth loads the signed-in user or bounces to the login page.
func (a *App) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := a.sessions.Get(r, sessionName)
		userID, ok := session.Values["user_id"].(int64)
		if !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		user, err := a.store.UserByID(userID)
		if err != nil || !user.IsActive {
			a.clearSession(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	}
}

func (a *App) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user := currentUser(r); user == nil || user.Role != core.RoleAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (a *App) clearSession(w http.ResponseWriter, r *http.Request) {
	session, _ := a.sessions.Get(r, sessionName)
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		log.Printf("clear session: %v", err)
	}
}

func (a *App) setFlash(w http.ResponseWriter, r *http.Request, message string) {
	session, _ := a.sessions.Get(r, sessionName)
	session.AddFlash(message)
	if err := session.Save(r, w); err != nil {
		log.Printf("set flash: %v", err)
	}
}

func (a *App) popFlash(w http.ResponseWriter, r *http.Request) string {
	session, _ := a.sessions.Get(r, sessionName)
	flashes := session.Flashes()
	if len(flashes) == 0 {
		return ""
	}
	if err := session.Save(r, w); err != nil {
		log.Printf("pop flash: %v", err)
	}
	message, _ := flashes[0].(string)
	return message
}

type loginPage struct {
	Email string
	Error string
	Next  string
}

// safeNext only follows local paths, never absolute URLs someone pasted
// into a link.
func safeNext(raw string) string {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return "/"
}

func (a *App) loginForm(w http.ResponseWriter, r *http.Request) {
	if session, _ := a.sessions.Get(r, sessionName); session.Values["user_id"] != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, r, http.StatusOK, "login", loginPage{Next: r.URL.Query().Get("next")})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := a.store.Authenticate(email, password)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if user == nil {
		a.render(w, r, http.StatusUnprocessableEntity, "login", loginPage{
			Email: email,
			Error: "invalid email or password",
			Next:  r.FormValue("next"),
		})
		return
	}
	session, _ := a.sessions.Get(r, sessionName)
	session.Values["user_id"] = user.ID
	if err := session.Save(r, w); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	session, _ := a.sessions.Get(r, sessionName)
	delete(session.Values, "user_id")
	session.AddFlash("Signed out.")
	if err := session.Save(r, w); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
