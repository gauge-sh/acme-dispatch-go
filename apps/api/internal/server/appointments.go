package server

import (
	"net/http"
	"time"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type appointmentPayload struct {
	JobID           Opt[int64]     `json:"job_id"`
	TechnicianID    Opt[int64]     `json:"technician_id"`
	ScheduledStart  Opt[core.Time] `json:"scheduled_start"`
	ScheduledEnd    Opt[core.Time] `json:"scheduled_end"`
	CompletionNotes Opt[string]    `json:"completion_notes"`
}

func (p appointmentPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "job_id", p.JobID.Set, p.JobID.Null)
	nullOn(fields, "technician_id", p.TechnicianID.Set, p.TechnicianID.Null)
	nullOn(fields, "scheduled_start", p.ScheduledStart.Set, p.ScheduledStart.Null)
	nullOn(fields, "scheduled_end", p.ScheduledEnd.Set, p.ScheduledEnd.Null)
	return fieldError(fields)
}

func (p appointmentPayload) requireTimes() error {
	fields := map[string]string{}
	if !p.ScheduledStart.Set {
		fields["scheduled_start"] = "is required"
	}
	if !p.ScheduledEnd.Set {
		fields["scheduled_end"] = "is required"
	}
	return fieldError(fields)
}

func (p appointmentPayload) applyTo(a *core.Appointment) {
	p.JobID.apply(&a.JobID)
	p.TechnicianID.apply(&a.TechnicianID)
	p.ScheduledStart.apply(&a.ScheduledStart)
	p.ScheduledEnd.apply(&a.ScheduledEnd)
	p.CompletionNotes.applyPtr(&a.CompletionNotes)
}

func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	var payload appointmentPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.requireTimes(); err != nil {
		writeError(w, err)
		return
	}
	var appointment core.Appointment
	payload.applyTo(&appointment)
	if err := s.store.CreateAppointment(&appointment); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appointment)
}

func (s *Server) getAppointment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "appointment not found")
		return
	}
	appointment, err := s.store.AppointmentByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appointment)
}

func (s *Server) patchAppointment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "appointment not found")
		return
	}
	appointment, err := s.store.AppointmentByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload appointmentPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(appointment)
	if err := s.store.UpdateAppointment(appointment); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appointment)
}

func (s *Server) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "appointment not found")
		return
	}
	if err := s.store.DeleteAppointment(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queryTime(r *http.Request, name string) (*core.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, core.Invalid(name + " must be an ISO-8601 timestamp")
	}
	t := core.NewTime(parsed)
	return &t, nil
}

func (s *Server) listAppointments(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var filter store.AppointmentFilter
	if filter.JobID, err = queryInt64(r, "job_id"); err != nil {
		writeError(w, err)
		return
	}
	if filter.TechnicianID, err = queryInt64(r, "technician_id"); err != nil {
		writeError(w, err)
		return
	}
	if filter.From, err = queryTime(r, "from"); err != nil {
		writeError(w, err)
		return
	}
	if filter.To, err = queryTime(r, "to"); err != nil {
		writeError(w, err)
		return
	}
	items, total, err := s.store.ListAppointments(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
