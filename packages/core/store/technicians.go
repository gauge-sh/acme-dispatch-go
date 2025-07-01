package store

import (
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

type TechnicianFilter struct {
	Skill  string
	Active *bool
}

func (s *Store) CreateTechnician(t *core.Technician) error {
	if err := core.ValidateTechnician(t); err != nil {
		return err
	}
	touch(&t.CreatedAt, &t.UpdatedAt)
	res, err := s.exec(`INSERT INTO technicians (name, phone, skill_tags, hourly_cost_cents, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.Phone, t.SkillTags, t.HourlyCostCents, t.IsActive, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return err
	}
	t.ID, err = res.LastInsertId()
	return err
}

func (s *Store) TechnicianByID(id int64) (*core.Technician, error) {
	var t core.Technician
	err := s.get(&t, `SELECT * FROM technicians WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("technician not found")
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) UpdateTechnician(t *core.Technician) error {
	if err := core.ValidateTechnician(t); err != nil {
		return err
	}
	t.UpdatedAt = core.Now()
	res, err := s.exec(`UPDATE technicians SET name = ?, phone = ?, skill_tags = ?, hourly_cost_cents = ?, is_active = ?, updated_at = ?
		WHERE id = ?`,
		t.Name, t.Phone, t.SkillTags, t.HourlyCostCents, t.IsActive, t.UpdatedAt, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("technician not found")
	}
	return nil
}

func (s *Store) DeleteTechnician(id int64) error {
	var appointments int
	if err := s.get(&appointments, `SELECT COUNT(*) FROM appointments WHERE technician_id = ?`, id); err != nil {
		return err
	}
	if appointments > 0 {
		return core.Invalid("technician has appointments and cannot be deleted")
	}
	res, err := s.exec(`DELETE FROM technicians WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("technician not found")
	}
	return nil
}

func (s *Store) ListTechnicians(f TechnicianFilter, p Page) ([]core.Technician, int, error) {
	var conds []string
	args := []any{}
	if f.Skill != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM json_each(technicians.skill_tags) WHERE json_each.value = ?)`)
		args = append(args, f.Skill)
	}
	if f.Active != nil {
		conds = append(conds, `is_active = ?`)
		args = append(args, *f.Active)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM technicians`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.Technician{}
	err := s.selectAll(&items, `SELECT * FROM technicians`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}
