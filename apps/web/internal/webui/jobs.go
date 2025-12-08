package webui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type jobFormPage struct {
	Job       *core.Job
	Customers []core.Customer
	Error     string
	Fields    map[string]string
}

func (a *App) jobFormPage(job *core.Job) (*jobFormPage, error) {
	customers, _, err := a.store.ListCustomers(store.CustomerFilter{}, store.Page{Number: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	return &jobFormPage{Job: job, Customers: customers}, nil
}

func (a *App) jobList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	filter := store.JobFilter{Query: formStr(r, "q")}
	if status, ok := core.ParseJobStatus(formStr(r, "status")); ok {
		filter.Statuses = []core.JobStatus{status}
	}
	if priority, ok := core.ParsePriority(formStr(r, "priority")); ok {
		filter.Priority = &priority
	}
	if customerID, err := strconv.ParseInt(formStr(r, "customer_id"), 10, 64); err == nil && customerID > 0 {
		filter.CustomerID = &customerID
	}
	jobs, total, err := a.store.ListJobs(filter, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	names, err := a.customerNames()
	if err != nil {
		a.serverError(w, err)
		return
	}
	customers, _, err := a.store.ListCustomers(store.CustomerFilter{}, store.Page{Number: 1, Size: 100})
	if err != nil {
		a.serverError(w, err)
		return
	}
	type row struct {
		Job          core.Job
		CustomerName string
	}
	rows := make([]row, 0, len(jobs))
	for _, job := range jobs {
		rows = append(rows, row{Job: job, CustomerName: names[job.CustomerID]})
	}
	a.render(w, r, http.StatusOK, "jobs", struct {
		Rows       []row
		Query      string
		Status     string
		Priority   string
		CustomerID string
		Customers  []core.Customer
		Statuses   []core.JobStatus
		Pager      pager
	}{rows, formStr(r, "q"), formStr(r, "status"), formStr(r, "priority"),
		formStr(r, "customer_id"), customers, core.JobStatuses, makePager(r, page, total)})
}

func (a *App) jobNew(w http.ResponseWriter, r *http.Request) {
	page, err := a.jobFormPage(&core.Job{Priority: core.PriorityNormal})
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "job_form", page)
}

func jobFromForm(r *http.Request, j *core.Job) {
	fields := map[string]string{}
	j.CustomerID = formInt(r, "customer_id", fields)
	j.Title = formStr(r, "title")
	j.Description = formOpt(r, "description")
	j.Priority = core.Priority(formStr(r, "priority"))
}

func (a *App) jobCreate(w http.ResponseWriter, r *http.Request) {
	var job core.Job
	jobFromForm(r, &job)
	if err := a.store.CreateJob(&job); err != nil {
		if message, fields, ok := formError(err); ok {
			page, pageErr := a.jobFormPage(&job)
			if pageErr != nil {
				a.serverError(w, pageErr)
				return
			}
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "job_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Job created.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", job.ID), http.StatusSeeOther)
}

type jobDetailPage struct {
	Job          *core.Job
	CustomerName string
	Appointments []appointmentRow
	JobParts     []jobPartRow
	InvoiceID    *int64
	NextStatuses []core.JobStatus
	CanInvoice   bool
	CanSchedule  bool
	CanAddParts  bool
	Parts        []store.StockWithPart
}

type appointmentRow struct {
	Appointment    core.Appointment
	TechnicianName string
}

type jobPartRow struct {
	JobPart core.JobPart
	SKU     string
	Name    string
}

func (a *App) jobDetailPage(job *core.Job) (*jobDetailPage, error) {
	customer, err := a.store.CustomerByID(job.CustomerID)
	if err != nil {
		return nil, err
	}
	appointments, err := a.store.AppointmentsForJob(job.ID)
	if err != nil {
		return nil, err
	}
	apptRows := make([]appointmentRow, 0, len(appointments))
	for _, appt := range appointments {
		tech, err := a.store.TechnicianByID(appt.TechnicianID)
		if err != nil {
			return nil, err
		}
		apptRows = append(apptRows, appointmentRow{Appointment: appt, TechnicianName: tech.Name})
	}
	jobParts, err := a.store.JobPartsForJob(job.ID)
	if err != nil {
		return nil, err
	}
	partRows := make([]jobPartRow, 0, len(jobParts))
	for _, jp := range jobParts {
		part, err := a.store.PartByID(jp.PartID)
		if err != nil {
			return nil, err
		}
		partRows = append(partRows, jobPartRow{JobPart: jp, SKU: part.SKU, Name: part.Name})
	}
	invoiceID, err := a.store.InvoiceRecordIDForJob(job.ID)
	if err != nil {
		return nil, err
	}
	stock, _, err := a.store.ListStockLevels(store.StockFilter{}, store.Page{Number: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	return &jobDetailPage{
		Job:          job,
		CustomerName: customer.Name,
		Appointments: apptRows,
		JobParts:     partRows,
		InvoiceID:    invoiceID,
		NextStatuses: core.NextStatuses(job.Status),
		CanInvoice:   job.Status == core.StatusCompleted,
		CanSchedule:  core.CheckAcceptsAppointments(job.Status) == nil,
		CanAddParts:  core.CheckAcceptsParts(job.Status) == nil,
		Parts:        stock,
	}, nil
}

func (a *App) jobDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	job, err := a.store.JobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page, err := a.jobDetailPage(job)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "job_detail", page)
}

func (a *App) jobEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	job, err := a.store.JobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page, err := a.jobFormPage(job)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "job_form", page)
}

func (a *App) jobUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	job, err := a.store.JobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	jobFromForm(r, job)
	if err := a.store.UpdateJob(job); err != nil {
		if message, fields, ok := formError(err); ok {
			page, pageErr := a.jobFormPage(job)
			if pageErr != nil {
				a.serverError(w, pageErr)
				return
			}
			page.Error, page.Fields = message, fields
			a.render(w, r, http.StatusUnprocessableEntity, "job_form", page)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Job updated.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
}

func (a *App) jobDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeleteJob(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Job deleted.")
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (a *App) jobTransition(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	target, ok := core.ParseJobStatus(formStr(r, "status"))
	if !ok {
		a.setFlash(w, r, "unknown status")
		http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
		return
	}
	if _, err := a.store.TransitionJob(id, target); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, fmt.Sprintf("Job moved to %s.", target))
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
}

func (a *App) jobPartAdd(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	fields := map[string]string{}
	partID := formInt(r, "part_id", fields)
	quantity := formInt(r, "quantity", fields)
	var err error
	if err = fieldsToError("part and quantity are required", fields); err == nil {
		_, err = a.store.AddJobPart(id, partID, quantity)
	}
	if err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Part recorded and taken from stock.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
}

func (a *App) jobPartRemove(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	jobPart, err := a.store.JobPartByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := a.store.RemoveJobPart(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d", jobPart.JobID), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Part returned to stock.")
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d", jobPart.JobID), http.StatusSeeOther)
}

func (a *App) jobInvoice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	record, err := a.store.CreateInvoiceRecord(id)
	if err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Invoice record created.")
	http.Redirect(w, r, fmt.Sprintf("/invoices/%d", record.ID), http.StatusSeeOther)
}
