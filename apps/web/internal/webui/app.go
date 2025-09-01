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

	mux.HandleFunc("GET /jobs", app.auth(app.jobList))
	mux.HandleFunc("GET /jobs/new", app.auth(app.jobNew))
	mux.HandleFunc("POST /jobs", app.auth(app.jobCreate))
	mux.HandleFunc("GET /jobs/{id}", app.auth(app.jobDetail))
	mux.HandleFunc("GET /jobs/{id}/edit", app.auth(app.jobEdit))
	mux.HandleFunc("POST /jobs/{id}", app.auth(app.jobUpdate))
	mux.HandleFunc("POST /jobs/{id}/delete", app.auth(app.jobDelete))
	mux.HandleFunc("POST /jobs/{id}/transition", app.auth(app.jobTransition))
	mux.HandleFunc("GET /jobs/{id}/appointments/new", app.auth(app.appointmentNew))
	mux.HandleFunc("POST /jobs/{id}/appointments", app.auth(app.appointmentCreate))
	mux.HandleFunc("POST /jobs/{id}/parts", app.auth(app.jobPartAdd))
	mux.HandleFunc("POST /jobs/{id}/invoice", app.auth(app.jobInvoice))
	mux.HandleFunc("GET /appointments/{id}/edit", app.auth(app.appointmentEdit))
	mux.HandleFunc("POST /appointments/{id}", app.auth(app.appointmentUpdate))
	mux.HandleFunc("POST /appointments/{id}/delete", app.auth(app.appointmentDelete))
	mux.HandleFunc("POST /job-parts/{id}/delete", app.auth(app.jobPartRemove))

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

	mux.HandleFunc("GET /parts", app.auth(app.partList))
	mux.HandleFunc("GET /parts/new", app.auth(app.partNew))
	mux.HandleFunc("POST /parts", app.auth(app.partCreate))
	mux.HandleFunc("GET /parts/{id}", app.auth(app.partDetail))
	mux.HandleFunc("GET /parts/{id}/edit", app.auth(app.partEdit))
	mux.HandleFunc("POST /parts/{id}", app.auth(app.partUpdate))
	mux.HandleFunc("POST /parts/{id}/delete", app.auth(app.requireAdmin(app.partDelete)))
	mux.HandleFunc("POST /parts/{id}/stock", app.auth(app.stockAdjust))

	mux.HandleFunc("GET /invoices", app.auth(app.invoiceList))
	mux.HandleFunc("GET /invoices/{id}", app.auth(app.invoiceDetail))

	return mux
}
