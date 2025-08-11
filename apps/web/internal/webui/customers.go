package webui

import (
	"fmt"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type customerFormPage struct {
	Customer *core.Customer
	Error    string
	Fields   map[string]string
}

func (a *App) customerList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	query := formStr(r, "q")
	customers, total, err := a.store.ListCustomers(store.CustomerFilter{Query: query}, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "customers", struct {
		Customers []core.Customer
		Query     string
		Pager     pager
	}{customers, query, makePager(r, page, total)})
}

func (a *App) customerNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "customer_form", customerFormPage{Customer: &core.Customer{}})
}

func customerFromForm(r *http.Request, c *core.Customer) {
	c.Name = formStr(r, "name")
	c.ServiceAddress = formStr(r, "service_address")
	c.ContactEmail = formOpt(r, "contact_email")
	c.Phone = formOpt(r, "phone")
	c.Notes = formOpt(r, "notes")
}

func (a *App) customerCreate(w http.ResponseWriter, r *http.Request) {
	var customer core.Customer
	customerFromForm(r, &customer)
	if err := a.store.CreateCustomer(&customer); err != nil {
		if message, fields, ok := formError(err); ok {
			a.render(w, r, http.StatusUnprocessableEntity, "customer_form",
				customerFormPage{Customer: &customer, Error: message, Fields: fields})
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Customer created.")
	http.Redirect(w, r, fmt.Sprintf("/customers/%d", customer.ID), http.StatusSeeOther)
}

func (a *App) customerDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	customer, err := a.store.CustomerByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	jobs, _, err := a.store.ListJobs(store.JobFilter{CustomerID: &id}, store.Page{Number: 1, Size: 100})
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "customer_detail", struct {
		Customer *core.Customer
		Jobs     []core.Job
	}{customer, jobs})
}

func (a *App) customerEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	customer, err := a.store.CustomerByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, http.StatusOK, "customer_form", customerFormPage{Customer: customer})
}

func (a *App) customerUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	customer, err := a.store.CustomerByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	customerFromForm(r, customer)
	if err := a.store.UpdateCustomer(customer); err != nil {
		if message, fields, ok := formError(err); ok {
			a.render(w, r, http.StatusUnprocessableEntity, "customer_form",
				customerFormPage{Customer: customer, Error: message, Fields: fields})
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Customer updated.")
	http.Redirect(w, r, fmt.Sprintf("/customers/%d", id), http.StatusSeeOther)
}

func (a *App) customerDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeleteCustomer(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/customers/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Customer deleted.")
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}
