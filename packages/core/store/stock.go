package store

import (
	"github.com/acme-dispatch/dispatch/core"
)

type StockFilter struct {
	PartID *int64
}

// StockWithPart is a stock level with its part attached, the shape both list
// screens and the API want.
type StockWithPart struct {
	core.StockLevel
	Part core.Part
}

// StockAdjustment is one entry in the audit trail: every change to a level,
// whether a manual adjustment or a job consuming parts, lands here.
type StockAdjustment struct {
	ID            int64     `db:"id" json:"id"`
	PartID        int64     `db:"part_id" json:"part_id"`
	Delta         int64     `db:"delta" json:"delta"`
	QuantityAfter int64     `db:"quantity_after" json:"quantity_after"`
	Reason        *string   `db:"reason" json:"reason"`
	CreatedAt     core.Time `db:"created_at" json:"created_at"`
}

func (s *Store) StockLevelForPart(partID int64) (*core.StockLevel, error) {
	var level core.StockLevel
	err := s.get(&level, `SELECT * FROM stock_levels WHERE part_id = ?`, partID)
	if noRows(err) {
		return nil, core.NotFound("part not found")
	}
	if err != nil {
		return nil, err
	}
	return &level, nil
}

func (s *Store) writeStockChange(level *core.StockLevel, newQuantity, delta int64, reason *string) error {
	level.QuantityOnHand = newQuantity
	level.UpdatedAt = core.Now()
	if _, err := s.exec(`UPDATE stock_levels SET quantity_on_hand = ?, updated_at = ? WHERE id = ?`,
		level.QuantityOnHand, level.UpdatedAt, level.ID); err != nil {
		return err
	}
	_, err := s.exec(`INSERT INTO stock_adjustments (part_id, delta, quantity_after, reason, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		level.PartID, delta, level.QuantityOnHand, reason, core.Now())
	return err
}

func (s *Store) AdjustStockLevel(partID, delta int64, reason *string) (*core.StockLevel, error) {
	var level *core.StockLevel
	err := s.InTx(func(tx *Store) error {
		var err error
		if level, err = tx.StockLevelForPart(partID); err != nil {
			return err
		}
		after, err := core.AdjustStock(level.QuantityOnHand, delta)
		if err != nil {
			return err
		}
		return tx.writeStockChange(level, after, delta, reason)
	})
	if err != nil {
		return nil, err
	}
	return level, nil
}

func (s *Store) ListStockLevels(f StockFilter, p Page) ([]StockWithPart, int, error) {
	where := ""
	args := []any{}
	if f.PartID != nil {
		where = ` WHERE part_id = ?`
		args = append(args, *f.PartID)
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM stock_levels`+where, args...); err != nil {
		return nil, 0, err
	}
	levels := []core.StockLevel{}
	err := s.selectAll(&levels, `SELECT * FROM stock_levels`+where+` ORDER BY part_id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	if err != nil {
		return nil, 0, err
	}

	items := make([]StockWithPart, 0, len(levels))
	for _, level := range levels {
		part, err := s.PartByID(level.PartID)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, StockWithPart{StockLevel: level, Part: *part})
	}
	return items, total, nil
}

func (s *Store) ListStockAdjustments(partID *int64, p Page) ([]StockAdjustment, int, error) {
	where := ""
	args := []any{}
	if partID != nil {
		where = ` WHERE part_id = ?`
		args = append(args, *partID)
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM stock_adjustments`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []StockAdjustment{}
	err := s.selectAll(&items, `SELECT * FROM stock_adjustments`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}
