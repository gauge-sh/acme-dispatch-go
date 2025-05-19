package store

import (
	"github.com/acme-dispatch/dispatch/core"
)

type PartFilter struct {
	Query string
}

func (s *Store) skuTaken(sku string, excludeID int64) (bool, error) {
	var n int
	err := s.get(&n, `SELECT COUNT(*) FROM parts WHERE sku = ? AND id != ?`, sku, excludeID)
	return n > 0, err
}

// CreatePart also opens the part's stock level at zero on hand; the two rows
// exist together or not at all.
func (s *Store) CreatePart(p *core.Part) error {
	if err := core.ValidatePart(p); err != nil {
		return err
	}
	return s.InTx(func(tx *Store) error {
		if taken, err := tx.skuTaken(p.SKU, 0); err != nil {
			return err
		} else if taken {
			return core.InvalidFields("sku is already in use", map[string]string{"sku": "is already in use"})
		}
		touch(&p.CreatedAt, &p.UpdatedAt)
		res, err := tx.exec(`INSERT INTO parts (sku, name, unit_cost_cents, unit_price_cents, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			p.SKU, p.Name, p.UnitCostCents, p.UnitPriceCents, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return err
		}
		if p.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		_, err = tx.exec(`INSERT INTO stock_levels (part_id, quantity_on_hand, created_at, updated_at)
			VALUES (?, 0, ?, ?)`, p.ID, p.CreatedAt, p.UpdatedAt)
		return err
	})
}

func (s *Store) PartByID(id int64) (*core.Part, error) {
	var p core.Part
	err := s.get(&p, `SELECT * FROM parts WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("part not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) UpdatePart(p *core.Part) error {
	if err := core.ValidatePart(p); err != nil {
		return err
	}
	if taken, err := s.skuTaken(p.SKU, p.ID); err != nil {
		return err
	} else if taken {
		return core.InvalidFields("sku is already in use", map[string]string{"sku": "is already in use"})
	}
	p.UpdatedAt = core.Now()
	res, err := s.exec(`UPDATE parts SET sku = ?, name = ?, unit_cost_cents = ?, unit_price_cents = ?, updated_at = ?
		WHERE id = ?`,
		p.SKU, p.Name, p.UnitCostCents, p.UnitPriceCents, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("part not found")
	}
	return nil
}

func (s *Store) DeletePart(id int64) error {
	var used int
	if err := s.get(&used, `SELECT COUNT(*) FROM job_parts WHERE part_id = ?`, id); err != nil {
		return err
	}
	if used > 0 {
		return core.Invalid("part is used on jobs and cannot be deleted")
	}
	res, err := s.exec(`DELETE FROM parts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("part not found")
	}
	return nil
}

func (s *Store) ListParts(f PartFilter, p Page) ([]core.Part, int, error) {
	where := ""
	args := []any{}
	if f.Query != "" {
		where = ` WHERE (sku LIKE ? OR name LIKE ?)`
		pattern := "%" + f.Query + "%"
		args = append(args, pattern, pattern)
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM parts`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.Part{}
	err := s.selectAll(&items, `SELECT * FROM parts`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}
