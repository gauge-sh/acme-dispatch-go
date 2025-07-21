package store

import (
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

type JobFilter struct {
	Statuses   []core.JobStatus
	Priority   *core.Priority
	CustomerID *int64
	Query      string
}

func (s *Store) CreateJob(j *core.Job) error {
	// Status is never writable on create; every job starts requested.
	j.Status = core.StatusRequested
	if j.Priority == "" {
		j.Priority = core.PriorityNormal
	}
	if err := core.ValidateJob(j); err != nil {
		return err
	}
	if _, err := s.CustomerByID(j.CustomerID); err != nil {
		return fieldMustExist(err, "customer_id")
	}
	touch(&j.CreatedAt, &j.UpdatedAt)
	res, err := s.exec(`INSERT INTO jobs (customer_id, title, description, priority, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		j.CustomerID, j.Title, j.Description, j.Priority, j.Status, j.CreatedAt, j.UpdatedAt)
	if err != nil {
		return err
	}
	j.ID, err = res.LastInsertId()
	return err
}

func (s *Store) JobByID(id int64) (*core.Job, error) {
	var j core.Job
	err := s.get(&j, `SELECT * FROM jobs WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("job not found")
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// UpdateJob writes everything but status, which only moves through
// TransitionJob or invoicing.
func (s *Store) UpdateJob(j *core.Job) error {
	if err := core.ValidateJob(j); err != nil {
		return err
	}
	if _, err := s.CustomerByID(j.CustomerID); err != nil {
		return fieldMustExist(err, "customer_id")
	}
	j.UpdatedAt = core.Now()
	res, err := s.exec(`UPDATE jobs SET customer_id = ?, title = ?, description = ?, priority = ?, updated_at = ?
		WHERE id = ?`,
		j.CustomerID, j.Title, j.Description, j.Priority, j.UpdatedAt, j.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("job not found")
	}
	return nil
}

func (s *Store) DeleteJob(id int64) error {
	return s.InTx(func(tx *Store) error {
		job, err := tx.JobByID(id)
		if err != nil {
			return err
		}
		if job.Status == core.StatusInvoiced {
			return core.Invalid("an invoiced job cannot be deleted")
		}
		_, err = tx.exec(`DELETE FROM jobs WHERE id = ?`, id)
		return err
	})
}

func (s *Store) TransitionJob(id int64, target core.JobStatus) (*core.Job, error) {
	var job *core.Job
	err := s.InTx(func(tx *Store) error {
		var err error
		if job, err = tx.JobByID(id); err != nil {
			return err
		}
		var appointments int
		if err := tx.get(&appointments, `SELECT COUNT(*) FROM appointments WHERE job_id = ?`, id); err != nil {
			return err
		}
		if err := core.CheckTransition(job.Status, target, appointments); err != nil {
			return err
		}
		job.Status = target
		job.UpdatedAt = core.Now()
		_, err = tx.exec(`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`, job.Status, job.UpdatedAt, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) ListJobs(f JobFilter, p Page) ([]core.Job, int, error) {
	var conds []string
	args := []any{}
	if len(f.Statuses) > 0 {
		placeholders := strings.Repeat("?, ", len(f.Statuses))
		conds = append(conds, "status IN ("+placeholders[:len(placeholders)-2]+")")
		for _, st := range f.Statuses {
			args = append(args, st)
		}
	}
	if f.Priority != nil {
		conds = append(conds, "priority = ?")
		args = append(args, *f.Priority)
	}
	if f.CustomerID != nil {
		conds = append(conds, "customer_id = ?")
		args = append(args, *f.CustomerID)
	}
	if f.Query != "" {
		conds = append(conds, "title LIKE ?")
		args = append(args, "%"+f.Query+"%")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM jobs`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.Job{}
	err := s.selectAll(&items, `SELECT * FROM jobs`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}

func (s *Store) JobStatusCounts() (map[core.JobStatus]int, error) {
	rows := []struct {
		Status core.JobStatus `db:"status"`
		N      int            `db:"n"`
	}{}
	if err := s.selectAll(&rows, `SELECT status, COUNT(*) AS n FROM jobs GROUP BY status`); err != nil {
		return nil, err
	}
	counts := map[core.JobStatus]int{}
	for _, st := range core.JobStatuses {
		counts[st] = 0
	}
	for _, r := range rows {
		counts[r.Status] = r.N
	}
	return counts, nil
}
