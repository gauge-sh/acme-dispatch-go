package store

import (
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

type AppointmentFilter struct {
	JobID        *int64
	TechnicianID *int64
	From         *core.Time
	To           *core.Time
}

// checkTechnicianConflict is the half-open overlap rule in SQL: RFC3339 UTC
// text at a fixed width compares correctly as strings. selfID excludes the
// appointment being rescheduled.
func (s *Store) checkTechnicianConflict(technicianID, selfID int64, start, end core.Time) error {
	var clashes int
	err := s.get(&clashes, `SELECT COUNT(*) FROM appointments
		WHERE technician_id = ? AND id != ? AND scheduled_start < ? AND scheduled_end > ?`,
		technicianID, selfID, end, start)
	if err != nil {
		return err
	}
	if clashes > 0 {
		return core.ScheduleConflict("technician is already booked for that time")
	}
	return nil
}

func (s *Store) validateAppointment(a *core.Appointment, forJob *core.Job) error {
	if err := core.CheckWindow(a.ScheduledStart.Time, a.ScheduledEnd.Time); err != nil {
		return err
	}
	if err := core.ValidateAppointmentNotes(a.CompletionNotes); err != nil {
		return err
	}
	if _, err := s.TechnicianByID(a.TechnicianID); err != nil {
		return err
	}
	if forJob != nil {
		if err := core.CheckAcceptsAppointments(forJob.Status); err != nil {
			return err
		}
	}
	return s.checkTechnicianConflict(a.TechnicianID, a.ID, a.ScheduledStart, a.ScheduledEnd)
}

func (s *Store) CreateAppointment(a *core.Appointment) error {
	return s.InTx(func(tx *Store) error {
		job, err := tx.JobByID(a.JobID)
		if err != nil {
			return err
		}
		if err := tx.validateAppointment(a, job); err != nil {
			return err
		}
		touch(&a.CreatedAt, &a.UpdatedAt)
		res, err := tx.exec(`INSERT INTO appointments (job_id, technician_id, scheduled_start, scheduled_end, completion_notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			a.JobID, a.TechnicianID, a.ScheduledStart, a.ScheduledEnd, a.CompletionNotes, a.CreatedAt, a.UpdatedAt)
		if err != nil {
			return err
		}
		a.ID, err = res.LastInsertId()
		return err
	})
}

func (s *Store) AppointmentByID(id int64) (*core.Appointment, error) {
	var a core.Appointment
	err := s.get(&a, `SELECT * FROM appointments WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("appointment not found")
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateAppointment re-checks the job gate only when the appointment moves to
// a different job: filling in completion notes after a job closes must work.
func (s *Store) UpdateAppointment(a *core.Appointment) error {
	return s.InTx(func(tx *Store) error {
		current, err := tx.AppointmentByID(a.ID)
		if err != nil {
			return err
		}
		var gate *core.Job
		if a.JobID != current.JobID {
			if gate, err = tx.JobByID(a.JobID); err != nil {
				return err
			}
		}
		if err := tx.validateAppointment(a, gate); err != nil {
			return err
		}
		a.UpdatedAt = core.Now()
		_, err = tx.exec(`UPDATE appointments SET job_id = ?, technician_id = ?, scheduled_start = ?, scheduled_end = ?, completion_notes = ?, updated_at = ?
			WHERE id = ?`,
			a.JobID, a.TechnicianID, a.ScheduledStart, a.ScheduledEnd, a.CompletionNotes, a.UpdatedAt, a.ID)
		return err
	})
}

func (s *Store) DeleteAppointment(id int64) error {
	res, err := s.exec(`DELETE FROM appointments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("appointment not found")
	}
	return nil
}

func (s *Store) ListAppointments(f AppointmentFilter, p Page) ([]core.Appointment, int, error) {
	var conds []string
	args := []any{}
	if f.JobID != nil {
		conds = append(conds, "job_id = ?")
		args = append(args, *f.JobID)
	}
	if f.TechnicianID != nil {
		conds = append(conds, "technician_id = ?")
		args = append(args, *f.TechnicianID)
	}
	if f.From != nil {
		conds = append(conds, "scheduled_start >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		conds = append(conds, "scheduled_start <= ?")
		args = append(args, *f.To)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM appointments`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.Appointment{}
	err := s.selectAll(&items, `SELECT * FROM appointments`+where+` ORDER BY scheduled_start, id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}

func (s *Store) AppointmentsForJob(jobID int64) ([]core.Appointment, error) {
	items := []core.Appointment{}
	err := s.selectAll(&items, `SELECT * FROM appointments WHERE job_id = ? ORDER BY scheduled_start, id`, jobID)
	return items, err
}
