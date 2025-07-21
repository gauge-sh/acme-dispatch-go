package server

import (
	"net/http"
	"slices"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type Server struct {
	store *store.Store
}

func New(st *store.Store) http.Handler {
	s := &Server{store: st}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/customers", s.listCustomers)
	mux.HandleFunc("POST /api/v1/customers", s.createCustomer)
	mux.HandleFunc("GET /api/v1/customers/{id}", s.getCustomer)
	mux.HandleFunc("PATCH /api/v1/customers/{id}", s.patchCustomer)
	mux.HandleFunc("DELETE /api/v1/customers/{id}", s.deleteCustomer)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeErrorCode(w, http.StatusNotFound, "not_found", "not found")
	})

	return s.authenticate(mux)
}

// fieldError folds per-field messages into one validation error, using the
// first field alphabetically as the headline.
func fieldError(fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return core.InvalidFields(keys[0]+" "+fields[keys[0]], fields)
}
