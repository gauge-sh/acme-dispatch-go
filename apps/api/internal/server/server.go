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

	mux.HandleFunc("GET /api/v1/technicians", s.listTechnicians)
	mux.HandleFunc("POST /api/v1/technicians", s.createTechnician)
	mux.HandleFunc("GET /api/v1/technicians/{id}", s.getTechnician)
	mux.HandleFunc("PATCH /api/v1/technicians/{id}", s.patchTechnician)
	mux.HandleFunc("DELETE /api/v1/technicians/{id}", s.requireAdmin(s.deleteTechnician))

	mux.HandleFunc("GET /api/v1/parts", s.listParts)
	mux.HandleFunc("POST /api/v1/parts", s.createPart)
	mux.HandleFunc("GET /api/v1/parts/{id}", s.getPart)
	mux.HandleFunc("PATCH /api/v1/parts/{id}", s.patchPart)
	mux.HandleFunc("DELETE /api/v1/parts/{id}", s.requireAdmin(s.deletePart))

	mux.HandleFunc("GET /api/v1/jobs", s.listJobs)
	mux.HandleFunc("POST /api/v1/jobs", s.createJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.getJob)
	mux.HandleFunc("PATCH /api/v1/jobs/{id}", s.patchJob)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", s.deleteJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/transition", s.transitionJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/parts", s.addJobPart)
	mux.HandleFunc("POST /api/v1/jobs/{id}/invoice", s.invoiceJob)
	mux.HandleFunc("DELETE /api/v1/job-parts/{id}", s.removeJobPart)

	mux.HandleFunc("GET /api/v1/stock-levels", s.listStockLevels)
	mux.HandleFunc("POST /api/v1/stock-levels/{part_id}/adjust", s.adjustStock)

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
