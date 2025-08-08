package webui

import (
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/acme-dispatch/dispatch/core/store"
)

type App struct {
	store    *store.Store
	sessions *sessions.CookieStore
}

func New(st *store.Store, sessionSecret []byte) http.Handler {
	cookies := sessions.NewCookieStore(sessionSecret)
	cookies.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   14 * 24 * 60 * 60,
	}
	app := &App{store: st, sessions: cookies}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", app.loginForm)
	mux.HandleFunc("POST /login", app.login)
	mux.HandleFunc("POST /logout", app.logout)
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))

	mux.HandleFunc("GET /{$}", app.auth(app.dashboard))

	return mux
}
