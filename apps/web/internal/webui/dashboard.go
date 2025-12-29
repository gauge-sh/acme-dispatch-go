package webui

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type dashboardJob struct {
	ID           int64
	Title        string
	Priority     core.Priority
	CustomerName string
}

type dashboardColumn struct {
	Status core.JobStatus
	Count  int
	Jobs   []dashboardJob
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	counts, err := a.store.JobStatusCounts()
	if err != nil {
		a.serverError(w, err)
		return
	}
	names, err := a.customerNames()
	if err != nil {
		a.serverError(w, err)
		return
	}
	columns := make([]dashboardColumn, 0, len(core.JobStatuses))
	for _, status := range core.JobStatuses {
		jobs, _, err := a.store.ListJobs(
			store.JobFilter{Statuses: []core.JobStatus{status}},
			store.Page{Number: 1, Size: 6})
		if err != nil {
			a.serverError(w, err)
			return
		}
		column := dashboardColumn{Status: status, Count: counts[status]}
		for _, job := range jobs {
			column.Jobs = append(column.Jobs, dashboardJob{
				ID:           job.ID,
				Title:        job.Title,
				Priority:     job.Priority,
				CustomerName: names[job.CustomerID],
			})
		}
		columns = append(columns, column)
	}
	a.render(w, r, http.StatusOK, "dashboard", struct{ Columns []dashboardColumn }{columns})
}

// customerNames is a lookup for list screens; the shop's customer book fits
// in one page.
func (a *App) customerNames() (map[int64]string, error) {
	customers, _, err := a.store.ListCustomers(store.CustomerFilter{}, store.Page{Number: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(customers))
	for _, c := range customers {
		names[c.ID] = c.Name
	}
	return names, nil
}
