package server

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core/store"
)

func (s *Server) getInvoiceRecord(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "invoice record not found")
		return
	}
	record, err := s.store.InvoiceRecordByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) listInvoiceRecords(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var filter store.InvoiceFilter
	if filter.JobID, err = queryInt64(r, "job_id"); err != nil {
		writeError(w, err)
		return
	}
	if filter.CustomerID, err = queryInt64(r, "customer_id"); err != nil {
		writeError(w, err)
		return
	}
	items, total, err := s.store.ListInvoiceRecords(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
