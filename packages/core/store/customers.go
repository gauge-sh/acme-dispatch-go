package store

import (
	"github.com/acme-dispatch/dispatch/core"
)

type CustomerFilter struct {
	Query string
}

func (s *Store) CreateCustomer(c *core.Customer) error {
	if err := core.ValidateCustomer(c); err != nil {
		return err
	}
	touch(&c.CreatedAt, &c.UpdatedAt)
	res, err := s.exec(`INSERT INTO customers (name, contact_email, phone, service_address, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.ContactEmail, c.Phone, c.ServiceAddress, c.Notes, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (s *Store) CustomerByID(id int64) (*core.Customer, error) {
	var c core.Customer
	err := s.get(&c, `SELECT * FROM customers WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("customer not found")
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateCustomer(c *core.Customer) error {
	if err := core.ValidateCustomer(c); err != nil {
		return err
	}
	c.UpdatedAt = core.Now()
	res, err := s.exec(`UPDATE customers SET name = ?, contact_email = ?, phone = ?, service_address = ?, notes = ?, updated_at = ?
		WHERE id = ?`,
		c.Name, c.ContactEmail, c.Phone, c.ServiceAddress, c.Notes, c.UpdatedAt, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("customer not found")
	}
	return nil
}

func (s *Store) DeleteCustomer(id int64) error {
	var jobs int
	if err := s.get(&jobs, `SELECT COUNT(*) FROM jobs WHERE customer_id = ?`, id); err != nil {
		return err
	}
	if jobs > 0 {
		return core.Invalid("customer has jobs and cannot be deleted")
	}
	res, err := s.exec(`DELETE FROM customers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("customer not found")
	}
	return nil
}

func (s *Store) ListCustomers(f CustomerFilter, p Page) ([]core.Customer, int, error) {
	where := ""
	args := []any{}
	if f.Query != "" {
		where = ` WHERE (name LIKE ? OR service_address LIKE ?)`
		pattern := "%" + f.Query + "%"
		args = append(args, pattern, pattern)
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM customers`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.Customer{}
	err := s.selectAll(&items, `SELECT * FROM customers`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}
