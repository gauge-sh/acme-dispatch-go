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

	mux.HandleFunc("GET /customers", app.auth(app.customerList))
	mux.HandleFunc("GET /customers/new", app.auth(app.customerNew))
	mux.HandleFunc("POST /customers", app.auth(app.customerCreate))
	mux.HandleFunc("GET /customers/{id}", app.auth(app.customerDetail))
	mux.HandleFunc("GET /customers/{id}/edit", app.auth(app.customerEdit))
	mux.HandleFunc("POST /customers/{id}", app.auth(app.customerUpdate))
	mux.HandleFunc("POST /customers/{id}/delete", app.auth(app.customerDelete))

	mux.HandleFunc("GET /technicians", app.auth(app.technicianList))
	mux.HandleFunc("GET /technicians/new", app.auth(app.technicianNew))
	mux.HandleFunc("POST /technicians", app.auth(app.technicianCreate))
	mux.HandleFunc("GET /technicians/{id}", app.auth(app.technicianDetail))
	mux.HandleFunc("GET /technicians/{id}/edit", app.auth(app.technicianEdit))
	mux.HandleFunc("POST /technicians/{id}", app.auth(app.technicianUpdate))
	mux.HandleFunc("POST /technicians/{id}/delete", app.auth(app.requireAdmin(app.technicianDelete)))

	return mux
}
