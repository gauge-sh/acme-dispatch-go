package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Time is a UTC timestamp at second precision. It goes on the wire as
// ISO-8601 with a Z suffix and into SQLite as the same text, so the database
// sorts it correctly. The zero value serializes as null.
type Time struct {
	time.Time
}

func NewTime(t time.Time) Time {
	return Time{t.UTC().Truncate(time.Second)}
}

func Now() Time {
	return NewTime(time.Now())
}

func (t Time) String() string {
	return t.UTC().Format(time.RFC3339)
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.String())
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var raw *string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		*t = Time{}
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return err
	}
	*t = NewTime(parsed)
	return nil
}

func (t Time) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.String(), nil
}

func (t *Time) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*t = Time{}
		return nil
	case time.Time:
		*t = NewTime(v)
		return nil
	case string:
		return t.scanText(v)
	case []byte:
		return t.scanText(string(v))
	}
	return fmt.Errorf("cannot scan %T into core.Time", src)
}

func (t *Time) scanText(s string) error {
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*t = NewTime(parsed)
	return nil
}
