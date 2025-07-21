package server

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type customerPayload struct {
	Name           Opt[string] `json:"name"`
	ContactEmail   Opt[string] `json:"contact_email"`
	Phone          Opt[string] `json:"phone"`
	ServiceAddress Opt[string] `json:"service_address"`
	Notes          Opt[string] `json:"notes"`
}

func (p customerPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "name", p.Name.Set, p.Name.Null)
	nullOn(fields, "service_address", p.ServiceAddress.Set, p.ServiceAddress.Null)
	return fieldError(fields)
}

func (p customerPayload) applyTo(c *core.Customer) {
	p.Name.apply(&c.Name)
	p.ServiceAddress.apply(&c.ServiceAddress)
	p.ContactEmail.applyPtr(&c.ContactEmail)
	p.Phone.applyPtr(&c.Phone)
	p.Notes.applyPtr(&c.Notes)
}

func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	var payload customerPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	var customer core.Customer
	payload.applyTo(&customer)
	if err := s.store.CreateCustomer(&customer); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, customer)
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	customer, err := s.store.CustomerByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (s *Server) patchCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	customer, err := s.store.CustomerByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload customerPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(customer)
	if err := s.store.UpdateCustomer(customer); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (s *Server) deleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	if err := s.store.DeleteCustomer(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter := store.CustomerFilter{Query: r.URL.Query().Get("q")}
	items, total, err := s.store.ListCustomers(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
