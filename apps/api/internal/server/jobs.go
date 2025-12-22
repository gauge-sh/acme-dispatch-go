package server

import (
	"net/http"
	"strings"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

// jobPayload has no status field on purpose: status in a POST or PATCH body
// is ignored, per the contract, and only moves through transitions.
type jobPayload struct {
	CustomerID  Opt[int64]         `json:"customer_id"`
	Title       Opt[string]        `json:"title"`
	Description Opt[string]        `json:"description"`
	Priority    Opt[core.Priority] `json:"priority"`
}

func (p jobPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "customer_id", p.CustomerID.Set, p.CustomerID.Null)
	nullOn(fields, "title", p.Title.Set, p.Title.Null)
	nullOn(fields, "priority", p.Priority.Set, p.Priority.Null)
	return fieldError(fields)
}

func (p jobPayload) applyTo(j *core.Job) {
	p.CustomerID.apply(&j.CustomerID)
	p.Title.apply(&j.Title)
	p.Description.applyPtr(&j.Description)
	p.Priority.apply(&j.Priority)
}

// jobDetail is the nested shape the detail route returns; the list route
// stays flat.
type jobDetail struct {
	ID           int64              `json:"id"`
	CustomerID   int64              `json:"customer_id"`
	Title        string             `json:"title"`
	Description  *string            `json:"description"`
	Priority     core.Priority      `json:"priority"`
	Status       core.JobStatus     `json:"status"`
	Appointments []core.Appointment `json:"appointments"`
	JobParts     []core.JobPart     `json:"job_parts"`
	InvoiceID    *int64             `json:"invoice_record_id"`
	CreatedAt    core.Time          `json:"created_at"`
	UpdatedAt    core.Time          `json:"updated_at"`
}

func (s *Server) jobDetail(job *core.Job) (*jobDetail, error) {
	appointments, err := s.store.AppointmentsForJob(job.ID)
	if err != nil {
		return nil, err
	}
	jobParts, err := s.store.JobPartsForJob(job.ID)
	if err != nil {
		return nil, err
	}
	invoiceID, err := s.store.InvoiceRecordIDForJob(job.ID)
	if err != nil {
		return nil, err
	}
	return &jobDetail{
		ID:           job.ID,
		CustomerID:   job.CustomerID,
		Title:        job.Title,
		Description:  job.Description,
		Priority:     job.Priority,
		Status:       job.Status,
		Appointments: appointments,
		JobParts:     jobParts,
		InvoiceID:    invoiceID,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}, nil
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var payload jobPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	var job core.Job
	payload.applyTo(&job)
	if err := s.store.CreateJob(&job); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	job, err := s.store.JobByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	detail, err := s.jobDetail(job)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) patchJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	job, err := s.store.JobByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload jobPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(job)
	if err := s.store.UpdateJob(job); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	if err := s.store.DeleteJob(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) transitionJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	target, ok := core.ParseJobStatus(payload.Status)
	if !ok {
		writeError(w, core.Invalid("unknown status"))
		return
	}
	job, err := s.store.TransitionJob(id, target)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) addJobPart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	var payload struct {
		PartID   Opt[int64] `json:"part_id"`
		Quantity Opt[int64] `json:"quantity"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	jobPart, err := s.store.AddJobPart(id, payload.PartID.Value, payload.Quantity.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, jobPart)
}

func (s *Server) removeJobPart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job part")
		return
	}
	if err := s.store.RemoveJobPart(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) invoiceJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "job")
		return
	}
	record, err := s.store.CreateInvoiceRecord(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter := store.JobFilter{Query: r.URL.Query().Get("q")}
	if raw := r.URL.Query().Get("status"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			status, ok := core.ParseJobStatus(strings.TrimSpace(part))
			if !ok {
				writeError(w, core.Invalid("unknown status"))
				return
			}
			filter.Statuses = append(filter.Statuses, status)
		}
	}
	if raw := r.URL.Query().Get("priority"); raw != "" {
		priority, ok := core.ParsePriority(raw)
		if !ok {
			writeError(w, core.Invalid("unknown priority"))
			return
		}
		filter.Priority = &priority
	}
	if filter.CustomerID, err = queryInt64(r, "customer_id"); err != nil {
		writeError(w, err)
		return
	}
	items, total, err := s.store.ListJobs(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
