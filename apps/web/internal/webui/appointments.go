package webui

import (
	"fmt"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type appointmentFormPage struct {
	Appointment *core.Appointment
	JobID       int64
	JobTitle    string
	Technicians []core.Technician
	Error       string
	Fields      map[string]string
}

func (a *App) appointmentFormPage(appt *core.Appointment, jobID int64) (*appointmentFormPage, error) {
	job, err := a.store.JobByID(jobID)
	if err != nil {
		return nil, err
	}
	active := true
	technicians, _, err := a.store.ListTechnicians(
		store.TechnicianFilter{Active: &active}, store.Page{Number: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	return &appointmentFormPage{
		Appointment: appt,
		JobID:       job.ID,
		JobTitle:    job.Title,
		Technicians: technicians,
	}, nil
}

func (a *App) appointmentNew(w http.ResponseWriter, r *http.Request) {
	jobID, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	page, err := a.appointmentFormPage(&core.Appointment{JobID: jobID}, jobID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, http.StatusOK, "appointment_form", page)
}

func appointmentFromForm(r *http.Request, appt *core.Appointment) error {
	fields := map[string]string{}
	appt.TechnicianID = formInt(r, "technician_id", fields)
	appt.ScheduledStart = formTime(r, "scheduled_start", fields)
	appt.ScheduledEnd = formTime(r, "scheduled_end", fields)
	appt.CompletionNotes = formOpt(r, "completion_notes")
	return fieldsToError("the appointment needs a technician and a time window", fields)
}

func (a *App) appointmentCreate(w http.ResponseWriter, r *http.Request) {
	jobID, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	appt := core.Appointment{JobID: jobID}
	err := appointmentFromForm(r, &appt)
	if err == nil {
		err = a.store.CreateAppointment(&appt)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			page, pageErr := a.appointmentFormPage(&appt, jobID)
			if pageErr != nil {
				a.serverError(w, pageErr)
				return
			}
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "appointment_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Appointment scheduled.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", jobID), http.StatusSeeOther)
}

func (a *App) appointmentEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	appt, err := a.store.AppointmentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page, err := a.appointmentFormPage(appt, appt.JobID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "appointment_form", page)
}

func (a *App) appointmentUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	appt, err := a.store.AppointmentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = appointmentFromForm(r, appt)
	if err == nil {
		err = a.store.UpdateAppointment(appt)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			page, pageErr := a.appointmentFormPage(appt, appt.JobID)
			if pageErr != nil {
				a.serverError(w, pageErr)
				return
			}
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "appointment_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Appointment updated.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", appt.JobID), http.StatusSeeOther)
}

func (a *App) appointmentDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	appt, err := a.store.AppointmentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeleteAppointment(id); err != nil {
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Appointment removed.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", appt.JobID), http.StatusSeeOther)
}
