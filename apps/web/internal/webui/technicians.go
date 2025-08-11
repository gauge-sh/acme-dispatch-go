package webui

import (
	"fmt"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type technicianFormPage struct {
	Technician *core.Technician
	Tags       string
	Rate       string
	Error      string
	Fields     map[string]string
}

func technicianForm(t *core.Technician) technicianFormPage {
	page := technicianFormPage{Technician: t, Tags: joinTags(t.SkillTags)}
	if t.HourlyCostCents > 0 {
		page.Rate = fmt.Sprintf("%d.%02d", t.HourlyCostCents/100, t.HourlyCostCents%100)
	}
	return page
}

func joinTags(tags []string) string {
	out := ""
	for i, tag := range tags {
		if i > 0 {
			out += ", "
		}
		out += tag
	}
	return out
}

func (a *App) technicianList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	filter := store.TechnicianFilter{Skill: formStr(r, "skill")}
	switch formStr(r, "show") {
	case "active":
		active := true
		filter.Active = &active
	case "inactive":
		active := false
		filter.Active = &active
	}
	technicians, total, err := a.store.ListTechnicians(filter, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "technicians", struct {
		Technicians []core.Technician
		Skill       string
		Show        string
		Pager       pager
	}{technicians, filter.Skill, formStr(r, "show"), makePager(r, page, total)})
}

func (a *App) technicianNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "technician_form", technicianForm(&core.Technician{IsActive: true}))
}

func technicianFromForm(r *http.Request, t *core.Technician) error {
	fields := map[string]string{}
	t.Name = formStr(r, "name")
	t.Phone = formOpt(r, "phone")
	t.SkillTags = formTags(r, "skill_tags")
	t.HourlyCostCents = formCents(r, "hourly_cost", fields)
	t.IsActive = r.FormValue("is_active") == "on"
	return fieldsToError("hourly_cost must be a dollar amount", fields)
}

func (a *App) technicianCreate(w http.ResponseWriter, r *http.Request) {
	var tech core.Technician
	err := technicianFromForm(r, &tech)
	if err == nil {
		err = a.store.CreateTechnician(&tech)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			page := technicianForm(&tech)
			page.Tags = formStr(r, "skill_tags")
			page.Rate = formStr(r, "hourly_cost")
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "technician_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Technician created.")
	http.Redirect(w, r, fmt.Sprintf("/technicians/%d", tech.ID), http.StatusSeeOther)
}

func (a *App) technicianDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tech, err := a.store.TechnicianByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// The schedule view: upcoming and recent appointments with job context.
	appointments, _, err := a.store.ListAppointments(
		store.AppointmentFilter{TechnicianID: &id}, store.Page{Number: 1, Size: 100})
	if err != nil {
		a.serverError(w, err)
		return
	}
	type scheduleRow struct {
		Appointment core.Appointment
		JobTitle    string
		JobStatus   core.JobStatus
	}
	rows := make([]scheduleRow, 0, len(appointments))
	for _, appt := range appointments {
		job, err := a.store.JobByID(appt.JobID)
		if err != nil {
			a.serverError(w, err)
			return
		}
		rows = append(rows, scheduleRow{Appointment: appt, JobTitle: job.Title, JobStatus: job.Status})
	}
	a.render(w, r, http.StatusOK, "technician_detail", struct {
		Technician *core.Technician
		Schedule   []scheduleRow
	}{tech, rows})
}

func (a *App) technicianEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tech, err := a.store.TechnicianByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, http.StatusOK, "technician_form", technicianForm(tech))
}

func (a *App) technicianUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tech, err := a.store.TechnicianByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = technicianFromForm(r, tech)
	if err == nil {
		err = a.store.UpdateTechnician(tech)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			page := technicianForm(tech)
			page.Tags = formStr(r, "skill_tags")
			page.Rate = formStr(r, "hourly_cost")
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "technician_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Technician updated.")
	http.Redirect(w, r, fmt.Sprintf("/technicians/%d", id), http.StatusSeeOther)
}

func (a *App) technicianDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeleteTechnician(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/technicians/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Technician deleted.")
	http.Redirect(w, r, "/technicians", http.StatusSeeOther)
}
