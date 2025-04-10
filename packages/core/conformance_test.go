package core

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

type fixtureCase struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Priority     string `json:"priority"`
	Appointments []struct {
		Start                     time.Time `json:"start"`
		End                       time.Time `json:"end"`
		Hours                     float64   `json:"hours"`
		TechnicianHourlyCostCents int64     `json:"technician_hourly_cost_cents"`
		Weekend                   bool      `json:"weekend"`
	} `json:"appointments"`
	Parts []struct {
		Quantity       int64 `json:"quantity"`
		UnitPriceCents int64 `json:"unit_price_cents"`
	} `json:"parts"`
	Expected struct {
		LaborCents int64 `json:"labor_cents"`
		PartsCents int64 `json:"parts_cents"`
		TotalCents int64 `json:"total_cents"`
	} `json:"expected"`
}

func TestPricingConformance(t *testing.T) {
	raw, err := os.ReadFile("testdata/pricing-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []fixtureCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 6 {
		t.Fatalf("expected 6 conformance cases, found %d", len(fixture.Cases))
	}

	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			priority, ok := ParsePriority(c.Priority)
			if !ok {
				t.Fatalf("bad priority in fixture: %q", c.Priority)
			}
			var labor []LaborInput
			for i, a := range c.Appointments {
				in := LaborInput{
					AppointmentID:   int64(i + 1),
					TechnicianName:  "Tech",
					HourlyCostCents: a.TechnicianHourlyCostCents,
					Start:           a.Start,
					End:             a.End,
				}
				// The fixture's hours and weekend columns are derived from
				// start/end; make sure we derive them the same way.
				if got := a.End.Sub(a.Start).Hours(); got != a.Hours {
					t.Fatalf("appointment %d: derived %v hours, fixture says %v", i, got, a.Hours)
				}
				if got := IsWeekend(a.Start); got != a.Weekend {
					t.Fatalf("appointment %d: derived weekend=%v, fixture says %v", i, got, a.Weekend)
				}
				labor = append(labor, in)
			}
			var parts []PartInput
			for i, p := range c.Parts {
				parts = append(parts, PartInput{
					JobPartID:      int64(i + 1),
					SKU:            "SKU-1",
					Name:           "Part",
					Quantity:       p.Quantity,
					UnitPriceCents: p.UnitPriceCents,
				})
			}

			inv := PriceInvoice(priority, labor, parts)
			if inv.LaborCents != c.Expected.LaborCents {
				t.Errorf("labor_cents = %d, want %d", inv.LaborCents, c.Expected.LaborCents)
			}
			if inv.PartsCents != c.Expected.PartsCents {
				t.Errorf("parts_cents = %d, want %d", inv.PartsCents, c.Expected.PartsCents)
			}
			if inv.TotalCents != c.Expected.TotalCents {
				t.Errorf("total_cents = %d, want %d", inv.TotalCents, c.Expected.TotalCents)
			}
		})
	}
}
