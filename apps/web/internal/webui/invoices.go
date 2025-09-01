package webui

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type invoiceRow struct {
	Invoice      core.InvoiceRecord
	JobTitle     string
	CustomerName string
}

func (a *App) invoiceRow(record core.InvoiceRecord) (invoiceRow, error) {
	job, err := a.store.JobByID(record.JobID)
	if err != nil {
		return invoiceRow{}, err
	}
	customer, err := a.store.CustomerByID(job.CustomerID)
	if err != nil {
		return invoiceRow{}, err
	}
	return invoiceRow{Invoice: record, JobTitle: job.Title, CustomerName: customer.Name}, nil
}

func (a *App) invoiceList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	records, total, err := a.store.ListInvoiceRecords(store.InvoiceFilter{}, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	rows := make([]invoiceRow, 0, len(records))
	for _, record := range records {
		row, err := a.invoiceRow(record)
		if err != nil {
			a.serverError(w, err)
			return
		}
		rows = append(rows, row)
	}
	a.render(w, r, http.StatusOK, "invoices", struct {
		Rows  []invoiceRow
		Pager pager
	}{rows, makePager(r, page, total)})
}

func (a *App) invoiceDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	record, err := a.store.InvoiceRecordByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := a.invoiceRow(*record)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "invoice_detail", row)
}
