package core

import (
	"errors"
	"strings"
	"testing"
)

func str(s string) *string { return &s }

func validCustomer() Customer {
	return Customer{Name: "Harbor Coffee", ServiceAddress: "12 Pier Rd"}
}

func TestValidateCustomer(t *testing.T) {
	c := validCustomer()
	if err := ValidateCustomer(&c); err != nil {
		t.Fatalf("valid customer rejected: %v", err)
	}

	c = validCustomer()
	c.Name = "   "
	var derr *Error
	if err := ValidateCustomer(&c); !errors.As(err, &derr) || derr.Fields["name"] == "" {
		t.Errorf("blank name: %v", err)
	}

	c = validCustomer()
	c.Name = strings.Repeat("x", 161)
	if err := ValidateCustomer(&c); err == nil {
		t.Error("161-char name accepted")
	}
	c.Name = strings.Repeat("x", 160)
	if err := ValidateCustomer(&c); err != nil {
		t.Errorf("160-char name rejected: %v", err)
	}

	c = validCustomer()
	c.ContactEmail = str("nope")
	if err := ValidateCustomer(&c); !errors.As(err, &derr) || derr.Fields["contact_email"] == "" {
		t.Errorf("email without @: %v", err)
	}
	c.ContactEmail = nil
	if err := ValidateCustomer(&c); err != nil {
		t.Errorf("nil email is fine: %v", err)
	}

	c = validCustomer()
	c.Phone = str("123")
	if err := ValidateCustomer(&c); !errors.As(err, &derr) || derr.Fields["phone"] == "" {
		t.Errorf("short phone: %v", err)
	}

	c = validCustomer()
	c.Notes = str(strings.Repeat("n", 2001))
	if err := ValidateCustomer(&c); err == nil {
		t.Error("2001-char notes accepted")
	}

	c = validCustomer()
	c.Name = "  Trimmed  "
	if err := ValidateCustomer(&c); err != nil || c.Name != "Trimmed" {
		t.Errorf("name not trimmed: %q %v", c.Name, err)
	}
}

func TestValidateTechnician(t *testing.T) {
	tech := Technician{Name: "Ray", HourlyCostCents: 5500, SkillTags: StringList{"HVAC", " plumbing ", "hvac"}}
	if err := ValidateTechnician(&tech); err != nil {
		t.Fatalf("valid technician rejected: %v", err)
	}
	if len(tech.SkillTags) != 2 || tech.SkillTags[0] != "hvac" || tech.SkillTags[1] != "plumbing" {
		t.Errorf("tags not normalized: %v", tech.SkillTags)
	}

	tech = Technician{Name: "Ray", HourlyCostCents: 0}
	var derr *Error
	if err := ValidateTechnician(&tech); !errors.As(err, &derr) || derr.Fields["hourly_cost_cents"] == "" {
		t.Errorf("zero rate: %v", err)
	}

	tech = Technician{Name: "Ray", HourlyCostCents: 100, SkillTags: StringList{strings.Repeat("a", 41)}}
	if err := ValidateTechnician(&tech); err == nil {
		t.Error("41-char tag accepted")
	}
}

func TestValidateJob(t *testing.T) {
	j := Job{Title: "Fix furnace", Priority: PriorityNormal}
	if err := ValidateJob(&j); err != nil {
		t.Fatalf("valid job rejected: %v", err)
	}
	j = Job{Title: "", Priority: PriorityNormal}
	if err := ValidateJob(&j); err == nil {
		t.Error("blank title accepted")
	}
	j = Job{Title: "ok", Priority: Priority("asap")}
	var derr *Error
	if err := ValidateJob(&j); !errors.As(err, &derr) || derr.Fields["priority"] == "" {
		t.Errorf("bad priority: %v", err)
	}
	j = Job{Title: "ok", Priority: PriorityNormal, Description: str(strings.Repeat("d", 4001))}
	if err := ValidateJob(&j); err == nil {
		t.Error("4001-char description accepted")
	}
}

func TestValidatePart(t *testing.T) {
	p := Part{SKU: "IGN-4420", Name: "Hot surface igniter", UnitCostCents: 800, UnitPriceCents: 1299}
	if err := ValidatePart(&p); err != nil {
		t.Fatalf("valid part rejected: %v", err)
	}

	for _, sku := range []string{"", "lower-case", "HAS SPACE", "UNDER_SCORE", strings.Repeat("A", 41)} {
		p := Part{SKU: sku, Name: "n", UnitCostCents: 1, UnitPriceCents: 1}
		if err := ValidatePart(&p); err == nil {
			t.Errorf("sku %q accepted", sku)
		}
	}

	p = Part{SKU: "OK-1", Name: "n", UnitCostCents: 500, UnitPriceCents: 499}
	var derr *Error
	err := ValidatePart(&p)
	if !errors.As(err, &derr) || derr.Message != "unit price cannot be below unit cost" {
		t.Errorf("price below cost: %v", err)
	}
	// Selling at cost is allowed; the rule is price >= cost.
	p = Part{SKU: "OK-1", Name: "n", UnitCostCents: 500, UnitPriceCents: 500}
	if err := ValidatePart(&p); err != nil {
		t.Errorf("price == cost rejected: %v", err)
	}
	p = Part{SKU: "OK-1", Name: "n", UnitCostCents: 0, UnitPriceCents: 100}
	if err := ValidatePart(&p); err == nil {
		t.Error("zero cost accepted")
	}
}

func TestValidateUser(t *testing.T) {
	u := User{Email: "Dispatch@Example.com", Name: "Pat", Role: RoleDispatcher}
	if err := ValidateUser(&u); err != nil {
		t.Fatalf("valid user rejected: %v", err)
	}
	if u.Email != "dispatch@example.com" {
		t.Errorf("email not lower-cased: %q", u.Email)
	}
	u = User{Email: "no-at-sign", Name: "Pat", Role: RoleAdmin}
	if err := ValidateUser(&u); err == nil {
		t.Error("email without @ accepted")
	}
	u = User{Email: "a@b.c", Name: "Pat", Role: Role("owner")}
	if err := ValidateUser(&u); err == nil {
		t.Error("unknown role accepted")
	}
}

func TestValidateAppointmentNotes(t *testing.T) {
	if err := ValidateAppointmentNotes(nil); err != nil {
		t.Fatalf("nil notes: %v", err)
	}
	if err := ValidateAppointmentNotes(str(strings.Repeat("n", 2000))); err != nil {
		t.Fatalf("2000-char notes: %v", err)
	}
	if err := ValidateAppointmentNotes(str(strings.Repeat("n", 2001))); err == nil {
		t.Fatal("2001-char notes accepted")
	}
}

func TestOptionalFieldsAreTrimmed(t *testing.T) {
	c := validCustomer()
	c.Phone = str("  555-0100  ")
	c.Notes = str("   ")
	if err := ValidateCustomer(&c); err != nil {
		t.Fatal(err)
	}
	if c.Phone == nil || *c.Phone != "555-0100" {
		t.Errorf("phone not trimmed: %v", c.Phone)
	}
	if c.Notes != nil {
		t.Errorf("blank notes should fold to nil: %q", *c.Notes)
	}

	// Padding must not carry a too-short phone past the length check.
	c = validCustomer()
	c.Phone = str("  123  ")
	if err := ValidateCustomer(&c); err == nil {
		t.Error("padded short phone accepted")
	}
}
