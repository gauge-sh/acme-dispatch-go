package store

import (
	"fmt"

	"github.com/acme-dispatch/dispatch/core"
)

// AddJobPart records parts used on a job and takes them out of stock in the
// same transaction; when stock runs short nothing is written at all.
func (s *Store) AddJobPart(jobID, partID, quantity int64) (*core.JobPart, error) {
	var jp *core.JobPart
	err := s.InTx(func(tx *Store) error {
		job, err := tx.JobByID(jobID)
		if err != nil {
			return err
		}
		if err := core.CheckAcceptsParts(job.Status); err != nil {
			return err
		}
		if _, err := tx.PartByID(partID); err != nil {
			return err
		}
		level, err := tx.StockLevelForPart(partID)
		if err != nil {
			return err
		}
		after, err := core.ConsumeStock(level.QuantityOnHand, quantity)
		if err != nil {
			return err
		}
		reason := fmt.Sprintf("used on job %d", jobID)
		if err := tx.writeStockChange(level, after, -quantity, &reason); err != nil {
			return err
		}
		now := core.Now()
		res, err := tx.exec(`INSERT INTO job_parts (job_id, part_id, quantity, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			jobID, partID, quantity, now, now)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		jp = &core.JobPart{
			ID:        id,
			JobID:     jobID,
			PartID:    partID,
			Quantity:  quantity,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return jp, nil
}

func (s *Store) JobPartByID(id int64) (*core.JobPart, error) {
	var jp core.JobPart
	err := s.get(&jp, `SELECT * FROM job_parts WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("job part not found")
	}
	if err != nil {
		return nil, err
	}
	return &jp, nil
}

// RemoveJobPart puts the quantity back on the shelf in the same transaction.
func (s *Store) RemoveJobPart(id int64) error {
	return s.InTx(func(tx *Store) error {
		jp, err := tx.JobPartByID(id)
		if err != nil {
			return err
		}
		job, err := tx.JobByID(jp.JobID)
		if err != nil {
			return err
		}
		if job.Status == core.StatusInvoiced {
			return core.Invalid("an invoiced job cannot be changed")
		}
		level, err := tx.StockLevelForPart(jp.PartID)
		if err != nil {
			return err
		}
		after, err := core.ReleaseStock(level.QuantityOnHand, jp.Quantity)
		if err != nil {
			return err
		}
		reason := fmt.Sprintf("returned from job %d", jp.JobID)
		if err := tx.writeStockChange(level, after, jp.Quantity, &reason); err != nil {
			return err
		}
		_, err = tx.exec(`DELETE FROM job_parts WHERE id = ?`, id)
		return err
	})
}

func (s *Store) JobPartsForJob(jobID int64) ([]core.JobPart, error) {
	items := []core.JobPart{}
	err := s.selectAll(&items, `SELECT * FROM job_parts WHERE job_id = ? ORDER BY id`, jobID)
	return items, err
}
