package store

import (
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/acme-dispatch/dispatch/core"
)

type UserFilter struct {
	Role   *core.Role
	Active *bool
}

func (s *Store) emailTaken(email string, excludeID int64) (bool, error) {
	var n int
	err := s.get(&n, `SELECT COUNT(*) FROM users WHERE email = ? AND id != ?`, email, excludeID)
	return n > 0, err
}

func (s *Store) activeAdminCount() (int, error) {
	var n int
	err := s.get(&n, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = 1`)
	return n, err
}

func (s *Store) CreateUser(u *core.User, password string) error {
	if u.Role == "" {
		u.Role = core.RoleDispatcher
	}
	if err := core.ValidateUser(u); err != nil {
		return err
	}
	if taken, err := s.emailTaken(u.Email, 0); err != nil {
		return err
	} else if taken {
		return core.InvalidFields("email is already in use", map[string]string{"email": "is already in use"})
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.PasswordHash = string(hash)
	}
	touch(&u.CreatedAt, &u.UpdatedAt)
	res, err := s.exec(`INSERT INTO users (email, name, role, is_active, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Email, u.Name, u.Role, u.IsActive, u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return err
	}
	u.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UserByID(id int64) (*core.User, error) {
	var u core.User
	err := s.get(&u, `SELECT * FROM users WHERE id = ?`, id)
	if noRows(err) {
		return nil, core.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByEmail(email string) (*core.User, error) {
	var u core.User
	err := s.get(&u, `SELECT * FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	if noRows(err) {
		return nil, core.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUser enforces the self-service rules: nobody edits their own role or
// deactivates themselves, and the last active admin stays an active admin.
func (s *Store) UpdateUser(actorID int64, u *core.User) error {
	if err := core.ValidateUser(u); err != nil {
		return err
	}
	return s.InTx(func(tx *Store) error {
		current, err := tx.UserByID(u.ID)
		if err != nil {
			return err
		}
		if actorID == u.ID && u.Role != current.Role {
			return core.Invalid("you cannot change your own role")
		}
		if actorID == u.ID && current.IsActive && !u.IsActive {
			return core.Invalid("you cannot deactivate yourself")
		}
		losesAdmin := current.Role == core.RoleAdmin && current.IsActive &&
			(u.Role != core.RoleAdmin || !u.IsActive)
		if losesAdmin {
			admins, err := tx.activeAdminCount()
			if err != nil {
				return err
			}
			if admins <= 1 {
				return core.Invalid("cannot remove the last active admin")
			}
		}
		if taken, err := tx.emailTaken(u.Email, u.ID); err != nil {
			return err
		} else if taken {
			return core.InvalidFields("email is already in use", map[string]string{"email": "is already in use"})
		}
		u.UpdatedAt = core.Now()
		_, err = tx.exec(`UPDATE users SET email = ?, name = ?, role = ?, is_active = ?, updated_at = ?
			WHERE id = ?`,
			u.Email, u.Name, u.Role, u.IsActive, u.UpdatedAt, u.ID)
		return err
	})
}

func (s *Store) SetUserPassword(id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		string(hash), core.Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("user not found")
	}
	return nil
}

func (s *Store) DeleteUser(id int64) error {
	return s.InTx(func(tx *Store) error {
		u, err := tx.UserByID(id)
		if err != nil {
			return err
		}
		if u.Role == core.RoleAdmin && u.IsActive {
			admins, err := tx.activeAdminCount()
			if err != nil {
				return err
			}
			if admins <= 1 {
				return core.Invalid("cannot remove the last active admin")
			}
		}
		_, err = tx.exec(`DELETE FROM users WHERE id = ?`, id)
		return err
	})
}

func (s *Store) ListUsers(f UserFilter, p Page) ([]core.User, int, error) {
	var conds []string
	args := []any{}
	if f.Role != nil {
		conds = append(conds, "role = ?")
		args = append(args, *f.Role)
	}
	if f.Active != nil {
		conds = append(conds, "is_active = ?")
		args = append(args, *f.Active)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.get(&total, `SELECT COUNT(*) FROM users`+where, args...); err != nil {
		return nil, 0, err
	}
	items := []core.User{}
	err := s.selectAll(&items, `SELECT * FROM users`+where+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, p.limit(), p.offset())...)
	return items, total, err
}

// Authenticate returns nil without an error when the credentials are wrong
// or the user is inactive; the caller shows the same message either way.
func (s *Store) Authenticate(email, password string) (*core.User, error) {
	u, err := s.UserByEmail(email)
	if err != nil {
		if _, notFound := err.(*core.Error); notFound {
			return nil, nil
		}
		return nil, err
	}
	if !u.IsActive || u.PasswordHash == "" {
		return nil, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, nil
	}
	return u, nil
}
