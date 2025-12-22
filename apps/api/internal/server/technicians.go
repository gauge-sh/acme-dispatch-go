package server

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type technicianPayload struct {
	Name            Opt[string]   `json:"name"`
	Phone           Opt[string]   `json:"phone"`
	SkillTags       Opt[[]string] `json:"skill_tags"`
	HourlyCostCents Opt[int64]    `json:"hourly_cost_cents"`
	IsActive        Opt[bool]     `json:"is_active"`
}

func (p technicianPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "name", p.Name.Set, p.Name.Null)
	nullOn(fields, "skill_tags", p.SkillTags.Set, p.SkillTags.Null)
	nullOn(fields, "hourly_cost_cents", p.HourlyCostCents.Set, p.HourlyCostCents.Null)
	nullOn(fields, "is_active", p.IsActive.Set, p.IsActive.Null)
	return fieldError(fields)
}

func (p technicianPayload) applyTo(t *core.Technician) {
	p.Name.apply(&t.Name)
	p.Phone.applyPtr(&t.Phone)
	if p.SkillTags.Set && !p.SkillTags.Null {
		t.SkillTags = p.SkillTags.Value
	}
	p.HourlyCostCents.apply(&t.HourlyCostCents)
	p.IsActive.apply(&t.IsActive)
}

func (s *Server) createTechnician(w http.ResponseWriter, r *http.Request) {
	var payload technicianPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	tech := core.Technician{IsActive: true, SkillTags: core.StringList{}}
	payload.applyTo(&tech)
	if err := s.store.CreateTechnician(&tech); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tech)
}

func (s *Server) getTechnician(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "technician")
		return
	}
	tech, err := s.store.TechnicianByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tech)
}

func (s *Server) patchTechnician(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "technician")
		return
	}
	tech, err := s.store.TechnicianByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload technicianPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(tech)
	if err := s.store.UpdateTechnician(tech); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tech)
}

func (s *Server) deleteTechnician(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w, "technician")
		return
	}
	if err := s.store.DeleteTechnician(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listTechnicians(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	active, err := queryBool(r, "is_active")
	if err != nil {
		writeError(w, err)
		return
	}
	filter := store.TechnicianFilter{Skill: r.URL.Query().Get("skill"), Active: active}
	items, total, err := s.store.ListTechnicians(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}
