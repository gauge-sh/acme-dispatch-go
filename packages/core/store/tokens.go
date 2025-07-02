package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/acme-dispatch/dispatch/core"
)

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// IssueToken mints a bearer token for a user. The plaintext is returned
// exactly once, here; only its hash is stored.
func (s *Store) IssueToken(userID int64, name string) (string, *core.APIToken, error) {
	if err := core.ValidateTokenName(name); err != nil {
		return "", nil, err
	}
	if _, err := s.UserByID(userID); err != nil {
		return "", nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext := hex.EncodeToString(raw)
	now := core.Now()
	res, err := s.exec(`INSERT INTO api_tokens (user_id, name, token_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		userID, name, hashToken(plaintext), now, now)
	if err != nil {
		return "", nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", nil, err
	}
	token := &core.APIToken{
		ID:        id,
		UserID:    userID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return plaintext, token, nil
}

// UserForToken resolves a bearer token to its active user, or nil when the
// token is unknown or the user cannot act.
func (s *Store) UserForToken(plaintext string) (*core.User, error) {
	var row struct {
		TokenID int64 `db:"token_id"`
		core.User
	}
	err := s.get(&row, `SELECT api_tokens.id AS token_id, users.*
		FROM api_tokens JOIN users ON users.id = api_tokens.user_id
		WHERE api_tokens.token_hash = ?`, hashToken(plaintext))
	if noRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !row.User.IsActive {
		return nil, nil
	}
	if _, err := s.exec(`UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, core.Now(), row.TokenID); err != nil {
		return nil, err
	}
	return &row.User, nil
}

func (s *Store) TokensForUser(userID int64) ([]core.APIToken, error) {
	items := []core.APIToken{}
	err := s.selectAll(&items, `SELECT * FROM api_tokens WHERE user_id = ? ORDER BY id`, userID)
	return items, err
}

func (s *Store) DeleteToken(id int64) error {
	res, err := s.exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return core.NotFound("token not found")
	}
	return nil
}
