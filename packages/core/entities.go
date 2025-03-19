package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList is stored as a JSON array in a text column and always
// serializes as an array, never null.
type StringList []string

func (l StringList) Value() (driver.Value, error) {
	data, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (l *StringList) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*l = StringList{}
		return nil
	case string:
		return json.Unmarshal([]byte(v), l)
	case []byte:
		return json.Unmarshal(v, l)
	}
	return fmt.Errorf("cannot scan %T into core.StringList", src)
}

func (l StringList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(l))
}

type User struct {
	ID           int64  `db:"id" json:"id"`
	Email        string `db:"email" json:"email"`
	Name         string `db:"name" json:"name"`
	Role         Role   `db:"role" json:"role"`
	IsActive     bool   `db:"is_active" json:"is_active"`
	PasswordHash string `db:"password_hash" json:"-"`
	CreatedAt    Time   `db:"created_at" json:"created_at"`
	UpdatedAt    Time   `db:"updated_at" json:"updated_at"`
}

type APIToken struct {
	ID         int64  `db:"id" json:"id"`
	UserID     int64  `db:"user_id" json:"user_id"`
	Name       string `db:"name" json:"name"`
	TokenHash  string `db:"token_hash" json:"-"`
	LastUsedAt Time   `db:"last_used_at" json:"last_used_at"`
	CreatedAt  Time   `db:"created_at" json:"created_at"`
	UpdatedAt  Time   `db:"updated_at" json:"updated_at"`
}

type Customer struct {
	ID             int64   `db:"id" json:"id"`
	Name           string  `db:"name" json:"name"`
	ContactEmail   *string `db:"contact_email" json:"contact_email"`
	Phone          *string `db:"phone" json:"phone"`
	ServiceAddress string  `db:"service_address" json:"service_address"`
	Notes          *string `db:"notes" json:"notes"`
	CreatedAt      Time    `db:"created_at" json:"created_at"`
	UpdatedAt      Time    `db:"updated_at" json:"updated_at"`
}

type Technician struct {
	ID              int64      `db:"id" json:"id"`
	Name            string     `db:"name" json:"name"`
	Phone           *string    `db:"phone" json:"phone"`
	SkillTags       StringList `db:"skill_tags" json:"skill_tags"`
	HourlyCostCents int64      `db:"hourly_cost_cents" json:"hourly_cost_cents"`
	IsActive        bool       `db:"is_active" json:"is_active"`
	CreatedAt       Time       `db:"created_at" json:"created_at"`
	UpdatedAt       Time       `db:"updated_at" json:"updated_at"`
}

type Job struct {
	ID          int64     `db:"id" json:"id"`
	CustomerID  int64     `db:"customer_id" json:"customer_id"`
	Title       string    `db:"title" json:"title"`
	Description *string   `db:"description" json:"description"`
	Priority    Priority  `db:"priority" json:"priority"`
	Status      JobStatus `db:"status" json:"status"`
	CreatedAt   Time      `db:"created_at" json:"created_at"`
	UpdatedAt   Time      `db:"updated_at" json:"updated_at"`
}

type Appointment struct {
	ID              int64   `db:"id" json:"id"`
	JobID           int64   `db:"job_id" json:"job_id"`
	TechnicianID    int64   `db:"technician_id" json:"technician_id"`
	ScheduledStart  Time    `db:"scheduled_start" json:"scheduled_start"`
	ScheduledEnd    Time    `db:"scheduled_end" json:"scheduled_end"`
	CompletionNotes *string `db:"completion_notes" json:"completion_notes"`
	CreatedAt       Time    `db:"created_at" json:"created_at"`
	UpdatedAt       Time    `db:"updated_at" json:"updated_at"`
}

func (a Appointment) Booking() Booking {
	return Booking{
		ID:           a.ID,
		TechnicianID: a.TechnicianID,
		Start:        a.ScheduledStart.Time,
		End:          a.ScheduledEnd.Time,
	}
}

type Part struct {
	ID             int64  `db:"id" json:"id"`
	SKU            string `db:"sku" json:"sku"`
	Name           string `db:"name" json:"name"`
	UnitCostCents  int64  `db:"unit_cost_cents" json:"unit_cost_cents"`
	UnitPriceCents int64  `db:"unit_price_cents" json:"unit_price_cents"`
	CreatedAt      Time   `db:"created_at" json:"created_at"`
	UpdatedAt      Time   `db:"updated_at" json:"updated_at"`
}

type StockLevel struct {
	ID             int64 `db:"id" json:"id"`
	PartID         int64 `db:"part_id" json:"part_id"`
	QuantityOnHand int64 `db:"quantity_on_hand" json:"quantity_on_hand"`
	CreatedAt      Time  `db:"created_at" json:"created_at"`
	UpdatedAt      Time  `db:"updated_at" json:"updated_at"`
}

type JobPart struct {
	ID             int64 `db:"id" json:"id"`
	JobID          int64 `db:"job_id" json:"job_id"`
	PartID         int64 `db:"part_id" json:"part_id"`
	Quantity       int64 `db:"quantity" json:"quantity"`
	UnitPriceCents int64 `db:"unit_price_cents" json:"unit_price_cents"`
	CreatedAt      Time  `db:"created_at" json:"created_at"`
	UpdatedAt      Time  `db:"updated_at" json:"updated_at"`
}

type InvoiceRecord struct {
	ID         int64      `db:"id" json:"id"`
	JobID      int64      `db:"job_id" json:"job_id"`
	LaborCents int64      `db:"labor_cents" json:"labor_cents"`
	PartsCents int64      `db:"parts_cents" json:"parts_cents"`
	TotalCents int64      `db:"total_cents" json:"total_cents"`
	LineItems  []LineItem `db:"-" json:"line_items"`
	CreatedAt  Time       `db:"created_at" json:"created_at"`
	UpdatedAt  Time       `db:"updated_at" json:"updated_at"`
}

type LineItem struct {
	Kind        LineKind `json:"kind"`
	Description string   `json:"description"`
	Quantity    float64  `json:"quantity"`
	UnitCents   int64    `json:"unit_cents"`
	AmountCents int64    `json:"amount_cents"`
	Weekend     bool     `json:"weekend"`
}
