package store

import (
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

type InvoiceFilter struct {
	JobID      *int64
	CustomerID *int64
}

// CreateInvoiceRecord prices the job as it stands and freezes the result. The
// record insert and the move to invoiced share one transaction.
func (s *Store) CreateInvoiceRecord(jobID int64) (*core.InvoiceRecord, error) {
	var rec *core.InvoiceRecord
	err := s.InTx(func(tx *Store) error {
		job, err := tx.JobByID(jobID)
		if err != nil {
			return err
		}
		var existing int
		if err := tx.get(&existing, `SELECT COUNT(*) FROM invoice_records WHERE job_id = ?`, jobID); err != nil {
			return err
		}
		if err := core.CheckInvoiceable(job.Status, existing > 0); err != nil {
			return err
		}

		var laborRows []struct {
			ID              int64     `db:"id"`
			ScheduledStart  core.Time `db:"scheduled_start"`
			ScheduledEnd    core.Time `db:"scheduled_end"`
			Name            string    `db:"name"`
			HourlyCostCents int64     `db:"hourly_cost_cents"`
		}
		err = tx.selectAll(&laborRows, `SELECT a.id, a.scheduled_start, a.scheduled_end, t.name, t.hourly_cost_cents
			FROM appointments a JOIN technicians t ON t.id = a.technician_id
			WHERE a.job_id = ?`, jobID)
		if err != nil {
			return err
		}
		labor := make([]core.LaborInput, 0, len(laborRows))
		for _, r := range laborRows {
			labor = append(labor, core.LaborInput{
				AppointmentID:   r.ID,
				TechnicianName:  r.Name,
				HourlyCostCents: r.HourlyCostCents,
				Start:           r.ScheduledStart.Time,
				End:             r.ScheduledEnd.Time,
			})
		}

		var partRows []struct {
			ID             int64  `db:"id"`
			Quantity       int64  `db:"quantity"`
			UnitPriceCents int64  `db:"unit_price_cents"`
			SKU            string `db:"sku"`
			Name           string `db:"name"`
		}
		err = tx.selectAll(&partRows, `SELECT jp.id, jp.quantity, jp.unit_price_cents, p.sku, p.name
			FROM job_parts jp JOIN parts p ON p.id = jp.part_id
			WHERE jp.job_id = ?`, jobID)
		if err != nil {
			return err
		}
		parts := make([]core.PartInput, 0, len(partRows))
		for _, r := range partRows {
			parts = append(parts, core.PartInput{
				JobPartID:      r.ID,
				SKU:            r.SKU,
				Name:           r.Name,
				Quantity:       r.Quantity,
				UnitPriceCents: r.UnitPriceCents,
			})
		}

		priced := core.PriceInvoice(job.Priority, labor, parts)
		now := core.Now()
		res, err := tx.exec(`INSERT INTO invoice_records (job_id, labor_cents, parts_cents, total_cents, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			jobID, priced.LaborCents, priced.PartsCents, priced.TotalCents, now, now)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for i, line := range priced.LineItems {
			_, err := tx.exec(`INSERT INTO invoice_line_items (invoice_record_id, position, kind, description, quantity, unit_cents, amount_cents, weekend)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, i, line.Kind, line.Description, line.Quantity, line.UnitCents, line.AmountCents, line.Weekend)
			if err != nil {
				return err
			}
		}
		if _, err := tx.exec(`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
			core.StatusInvoiced, now, jobID); err != nil {
			return err
		}
		rec = &core.InvoiceRecord{
			ID:         id,
			JobID:      jobID,
			LaborCents: priced.LaborCents,
			PartsCents: priced.PartsCents,
			TotalCents: priced.TotalCents,
			LineItems:  priced.LineItems,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Store) lineItems(recordID int64) ([]core.LineItem, error) {
	items := []core.LineItem{}
	err := s.selectAll(&items, `SELECT kind, description, quantity, unit_cents, amount_cents, weekend
		FROM invoice_line_items WHERE invoice_record_id = ? ORDER BY position`, recordID)
	return items, err
}

func (s *Store) InvoiceRecordByID(id int64) (*core.InvoiceRecord, error) {
	var rec core.InvoiceRecord
	err := s.get(&rec, `SELECT * FROM invoice_records WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("invoice record not found")
	}
	if err != nil {
		return nil, err
	}
	if rec.LineItems, err = s.lineItems(rec.ID); err != nil {
		return nil, err
	}
	return &rec, nil
}

// InvoiceRecordIDForJob is nil when the job has not been invoiced.
func (s *Store) InvoiceRecordIDForJob(jobID int64) (*int64, error) {
	var id int64
	err := s.get(&id, `SELECT id FROM invoice_records WHERE job_id = ?`, jobID)
	if noRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (s *Store) ListInvoiceRecords(f InvoiceFilter, p Page) ([]core.InvoiceRecord, int, error) {
	var conds []string
	args := []any{}
	if f.JobID != nil {
		conds = append(conds, "invoice_records.job_id = ?")
		args = append(args, *f.JobID)
	}
	if f.CustomerID != nil {
		conds = append(conds, "invoice_records.job_id IN (SELECT id FROM jobs WHERE customer_id = ?)")
		args = append(args, *f.CustomerID)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM invoice_records`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.InvoiceRecord{}
	err := s.selectAll(&items, `SELECT * FROM invoice_records`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		if items[i].LineItems, err = s.lineItems(items[i].ID); err != nil {
			return nil, 0, err
		}
	}
	return items, total, err
}
