package core

import (
	"fmt"
	"slices"
	"strings"
)

func lengthBetween(s string, min, max int) bool {
	n := len([]rune(s))
	return n >= min && n <= max
}

func checkOptional(fields map[string]string, name string, value *string, check func(string) string) {
	if value == nil {
		return
	}
	if msg := check(*value); msg != "" {
		fields[name] = msg
	}
}

func requireRange(fields map[string]string, name, value string, min, max int) {
	if !lengthBetween(value, min, max) {
		fields[name] = fmt.Sprintf("must be %d to %d characters", min, max)
	}
}

// trimOptional trims an optional field and folds a blank value to nil, so
// "  " never sneaks past a presence check.
func trimOptional(value **string) {
	if *value == nil {
		return
	}
	trimmed := strings.TrimSpace(**value)
	if trimmed == "" {
		*value = nil
		return
	}
	*value = &trimmed
}

func emailShaped(s string) string {
	if !strings.Contains(s, "@") {
		return "must be an email address"
	}
	return ""
}

func phoneShaped(s string) string {
	if !lengthBetween(s, 7, 32) {
		return "must be 7 to 32 characters"
	}
	return ""
}

func failValidation(fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return InvalidFields(keys[0]+" "+fields[keys[0]], fields)
}

// ValidateCustomer trims free-text fields in place and reports every field
// that breaks a rule, not just the first.
func ValidateCustomer(c *Customer) error {
	c.Name = strings.TrimSpace(c.Name)
	c.ServiceAddress = strings.TrimSpace(c.ServiceAddress)
	trimOptional(&c.ContactEmail)
	trimOptional(&c.Phone)
	trimOptional(&c.Notes)
	fields := map[string]string{}
	requireRange(fields, "name", c.Name, 1, 160)
	requireRange(fields, "service_address", c.ServiceAddress, 1, 240)
	checkOptional(fields, "contact_email", c.ContactEmail, emailShaped)
	checkOptional(fields, "phone", c.Phone, phoneShaped)
	checkOptional(fields, "notes", c.Notes, func(s string) string {
		if !lengthBetween(s, 0, 2000) {
			return "must be at most 2000 characters"
		}
		return ""
	})
	return failValidation(fields)
}

// NormalizeSkillTags lower-cases, trims, deduplicates, and sorts. Order and
// case are storage details the caller should never have to think about.
func NormalizeSkillTags(tags []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if !lengthBetween(tag, 1, 40) {
			return nil, InvalidFields("skill_tags must each be 1 to 40 characters",
				map[string]string{"skill_tags": "each tag must be 1 to 40 characters"})
		}
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	slices.Sort(out)
	return out, nil
}

func ValidateTechnician(t *Technician) error {
	t.Name = strings.TrimSpace(t.Name)
	tags, err := NormalizeSkillTags(t.SkillTags)
	if err != nil {
		return err
	}
	t.SkillTags = tags
	trimOptional(&t.Phone)
	fields := map[string]string{}
	requireRange(fields, "name", t.Name, 1, 120)
	checkOptional(fields, "phone", t.Phone, phoneShaped)
	if t.HourlyCostCents <= 0 {
		fields["hourly_cost_cents"] = "must be greater than zero"
	}
	return failValidation(fields)
}

func ValidateJob(j *Job) error {
	j.Title = strings.TrimSpace(j.Title)
	fields := map[string]string{}
	requireRange(fields, "title", j.Title, 1, 160)
	checkOptional(fields, "description", j.Description, func(s string) string {
		if !lengthBetween(s, 0, 4000) {
			return "must be at most 4000 characters"
		}
		return ""
	})
	if _, ok := ParsePriority(string(j.Priority)); !ok {
		fields["priority"] = "must be low, normal, or urgent"
	}
	return failValidation(fields)
}

func ValidateAppointmentNotes(notes *string) error {
	fields := map[string]string{}
	checkOptional(fields, "completion_notes", notes, func(s string) string {
		if !lengthBetween(s, 0, 2000) {
			return "must be at most 2000 characters"
		}
		return ""
	})
	return failValidation(fields)
}

func validSKU(sku string) bool {
	if !lengthBetween(sku, 1, 40) {
		return false
	}
	for _, r := range sku {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

func ValidatePart(p *Part) error {
	p.SKU = strings.TrimSpace(p.SKU)
	p.Name = strings.TrimSpace(p.Name)
	fields := map[string]string{}
	if !validSKU(p.SKU) {
		fields["sku"] = "must be 1 to 40 upper-case alphanumerics and dashes"
	}
	requireRange(fields, "name", p.Name, 1, 160)
	if p.UnitCostCents <= 0 {
		fields["unit_cost_cents"] = "must be greater than zero"
	}
	if p.UnitPriceCents <= 0 {
		fields["unit_price_cents"] = "must be greater than zero"
	}
	if err := failValidation(fields); err != nil {
		return err
	}
	if p.UnitPriceCents < p.UnitCostCents {
		return InvalidFields("unit price cannot be below unit cost",
			map[string]string{"unit_price_cents": "cannot be below unit cost"})
	}
	return nil
}

func ValidateUser(u *User) error {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.Name = strings.TrimSpace(u.Name)
	fields := map[string]string{}
	if msg := emailShaped(u.Email); msg != "" {
		fields["email"] = msg
	}
	requireRange(fields, "name", u.Name, 1, 120)
	if _, ok := ParseRole(string(u.Role)); !ok {
		fields["role"] = "must be admin or dispatcher"
	}
	return failValidation(fields)
}

func ValidateTokenName(name string) error {
	fields := map[string]string{}
	requireRange(fields, "name", strings.TrimSpace(name), 1, 80)
	return failValidation(fields)
}
