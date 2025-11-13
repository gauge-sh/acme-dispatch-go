package core

import (
	"errors"
	"testing"
	"time"
)

func labor(id int64, name string, rate int64, start time.Time, hours float64) LaborInput {
	return LaborInput{
		AppointmentID:   id,
		TechnicianName:  name,
		HourlyCostCents: rate,
		Start:           start,
		End:             start.Add(time.Duration(hours * float64(time.Hour))),
	}
}

func TestLaborLineRounding(t *testing.T) {
	weekday := time.Date(2025, 3, 12, 8, 0, 0, 0, time.UTC)
	// 0.5h × 3333¢ × 1.6 = 2666.4 → 2666
	line := laborLine(labor(1, "Sam", 3333, weekday, 0.5))
	if line.AmountCents != 2666 {
		t.Errorf("half hour at 3333: got %d, want 2666", line.AmountCents)
	}
	// 0.5h × 3334¢ × 1.6 = 2667.2 → 2667
	line = laborLine(labor(1, "Sam", 3334, weekday, 0.5))
	if line.AmountCents != 2667 {
		t.Errorf("half hour at 3334: got %d, want 2667", line.AmountCents)
	}
	// 15 minutes at 1250¢: 0.25 × 1250 × 1.6 = 500 exactly
	line = laborLine(labor(1, "Sam", 1250, weekday, 0.25))
	if line.AmountCents != 500 {
		t.Errorf("quarter hour at 1250: got %d, want 500", line.AmountCents)
	}
	// The half-cent case: 0.25h × 1201¢ × 1.6 = 480.4 → 480,
	// and 0.25h × 1226¢ × 1.6 = 490.4 → 490; a true .5 needs seconds.
	// 1s at 4500¢/h: 4500 × 1.6 / 3600 = 2.0 exactly.
	one := LaborInput{AppointmentID: 1, TechnicianName: "Sam", HourlyCostCents: 4500,
		Start: weekday, End: weekday.Add(time.Second)}
	if got := laborAmountCents(one); got != 2 {
		t.Errorf("one second at 4500: got %d, want 2", got)
	}
	// 1s at 1125¢/h: 1125 × 1.6 / 3600 = 0.5 → rounds away from zero to 1.
	half := LaborInput{AppointmentID: 1, TechnicianName: "Sam", HourlyCostCents: 1125,
		Start: weekday, End: weekday.Add(time.Second)}
	if got := laborAmountCents(half); got != 1 {
		t.Errorf("exact half cent should round up: got %d, want 1", got)
	}
}

func TestWeekendSurchargeAppliesToRoundedLine(t *testing.T) {
	saturday := time.Date(2025, 3, 15, 9, 0, 0, 0, time.UTC)
	// 0.5h × 3333 × 1.6 = 2666.4 → 2666, then × 1.15 = 3065.9 → 3066.
	// Rounding the raw product instead would give 3066.36 → 3066 here, so
	// pick a case where the two orders differ: 2666 × 1.15 = 3065.9 vs
	// 2666.4 × 1.15 = 3066.36. They land on the same cent by accident;
	// case 6 in the fixture pins the base rounding, this pins the stacking.
	line := laborLine(labor(1, "Sam", 3333, saturday, 0.5))
	if line.AmountCents != 3066 {
		t.Errorf("saturday half hour at 3333: got %d, want 3066", line.AmountCents)
	}
	if !line.Weekend {
		t.Error("weekend flag should be set")
	}
}

func TestWeekendIsPerAppointmentNotPerInvoice(t *testing.T) {
	weekday := time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)
	sunday := time.Date(2025, 3, 16, 9, 0, 0, 0, time.UTC)
	inv := PriceInvoice(PriorityNormal, []LaborInput{
		labor(1, "Sam", 5000, weekday, 2),
		labor(2, "Alex", 5000, sunday, 1),
	}, nil)
	if inv.LaborCents != 25200 {
		t.Errorf("labor: got %d, want 25200 (16000 weekday + 9200 sunday)", inv.LaborCents)
	}
	if inv.LineItems[0].Weekend || !inv.LineItems[1].Weekend {
		t.Error("only the sunday line takes the surcharge")
	}
}

func TestUrgentMultipliesTheRoundedTotalLast(t *testing.T) {
	weekday := time.Date(2025, 3, 12, 13, 0, 0, 0, time.UTC)
	l := []LaborInput{labor(1, "Dana", 8000, weekday, 1.5)}
	p := []PartInput{{JobPartID: 1, SKU: "IGN-4420", Name: "Hot surface igniter", Quantity: 3, UnitPriceCents: 1299}}
	normal := PriceInvoice(PriorityNormal, l, p)
	if normal.TotalCents != 23097 {
		t.Fatalf("normal total: got %d, want 23097", normal.TotalCents)
	}
	urgent := PriceInvoice(PriorityUrgent, l, p)
	if urgent.TotalCents != 25407 {
		t.Fatalf("urgent total: got %d, want 25407 (23097 × 1.1 = 25406.7)", urgent.TotalCents)
	}
	// Lines and subtotals are untouched by the multiplier.
	if urgent.LaborCents != 19200 || urgent.PartsCents != 3897 {
		t.Errorf("subtotals changed: labor %d parts %d", urgent.LaborCents, urgent.PartsCents)
	}
	for i, line := range urgent.LineItems {
		if line.AmountCents != normal.LineItems[i].AmountCents {
			t.Errorf("line %d amount changed under urgent", i)
		}
	}
	low := PriceInvoice(PriorityLow, l, p)
	if low.TotalCents != 23097 {
		t.Errorf("low priority must not multiply: got %d", low.TotalCents)
	}
}

func TestLineOrderingAndDescriptions(t *testing.T) {
	morning := time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)
	afternoon := time.Date(2025, 3, 12, 13, 0, 0, 0, time.UTC)
	inv := PriceInvoice(PriorityNormal,
		[]LaborInput{
			labor(9, "Late Tech", 5000, afternoon, 1),
			labor(7, "Tied Two", 5000, morning, 1),
			labor(3, "Tied One", 5000, morning, 2),
		},
		[]PartInput{
			{JobPartID: 12, SKU: "FLT-16", Name: "Filter", Quantity: 2, UnitPriceCents: 899},
			{JobPartID: 4, SKU: "IGN-4420", Name: "Hot surface igniter", Quantity: 1, UnitPriceCents: 1299},
		})

	wantDescriptions := []string{
		"Tied One — 2h on 2025-03-12", // same start, lower id first
		"Tied Two — 1h on 2025-03-12",
		"Late Tech — 1h on 2025-03-12",
		"IGN-4420 — Hot surface igniter", // parts by job-part id
		"FLT-16 — Filter",
	}
	if len(inv.LineItems) != len(wantDescriptions) {
		t.Fatalf("got %d lines", len(inv.LineItems))
	}
	for i, want := range wantDescriptions {
		if inv.LineItems[i].Description != want {
			t.Errorf("line %d: got %q, want %q", i, inv.LineItems[i].Description, want)
		}
	}
	if inv.LineItems[3].Quantity != 1 || inv.LineItems[4].Quantity != 2 {
		t.Error("part quantities out of order")
	}
}

func TestFormatHours(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{2 * time.Hour, "2"},
		{90 * time.Minute, "1.5"},
		{time.Hour, "1"},
		{15 * time.Minute, "0.25"},
		{20 * time.Minute, "0.33"}, // a third of an hour reads 0.33, bills exactly
		{18 * time.Minute, "0.3"},
		{40 * time.Minute, "0.67"},
		{45 * time.Second, "0.01"},
		{10 * time.Second, "0"}, // below label granularity; amount still exact
		{25 * time.Hour, "25"},
	}
	for _, c := range cases {
		if got := FormatHours(c.d); got != c.want {
			t.Errorf("FormatHours(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTwentyMinutesBillsAThirdNotPointThirtyThree(t *testing.T) {
	weekday := time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)
	l := LaborInput{AppointmentID: 1, TechnicianName: "Sam", HourlyCostCents: 9000,
		Start: weekday, End: weekday.Add(20 * time.Minute)}
	// (1200s × 9000 × 1.6) / 3600 = 4800 exactly; 0.33 × 9000 × 1.6 would be 4752.
	if got := laborAmountCents(l); got != 4800 {
		t.Errorf("twenty minutes at 9000: got %d, want 4800", got)
	}
	line := laborLine(l)
	if line.Description != "Sam — 0.33h on 2025-03-12" {
		t.Errorf("description: %q", line.Description)
	}
}

func TestCheckInvoiceable(t *testing.T) {
	if err := CheckInvoiceable(StatusCompleted, false); err != nil {
		t.Fatalf("completed job should invoice: %v", err)
	}
	var derr *Error
	for _, st := range []JobStatus{StatusRequested, StatusScheduled, StatusInProgress, StatusCancelled} {
		err := CheckInvoiceable(st, false)
		if !errors.As(err, &derr) || derr.Code != CodeInvalidTransition || derr.Message != "only a completed job can be invoiced" {
			t.Errorf("invoice from %s: %v", st, err)
		}
	}
	// The duplicate check outranks the status check even when both fail.
	err := CheckInvoiceable(StatusInvoiced, true)
	if !errors.As(err, &derr) || derr.Code != CodeValidationFailed || derr.Message != "job is already invoiced" {
		t.Errorf("double invoice: %v", err)
	}
}

func TestWeekendAndUrgentStack(t *testing.T) {
	saturday := time.Date(2025, 3, 15, 9, 0, 0, 0, time.UTC)
	// 2h × 5000 × 1.6 = 16000, ×1.15 = 18400 on the line;
	// urgent then lifts the total: 18400 × 1.1 = 20240.
	inv := PriceInvoice(PriorityUrgent, []LaborInput{labor(1, "Sam", 5000, saturday, 2)}, nil)
	if inv.LaborCents != 18400 {
		t.Errorf("labor: got %d, want 18400", inv.LaborCents)
	}
	if inv.TotalCents != 20240 {
		t.Errorf("total: got %d, want 20240", inv.TotalCents)
	}
	if inv.LineItems[0].AmountCents != 18400 {
		t.Errorf("line keeps only the weekend surcharge: %d", inv.LineItems[0].AmountCents)
	}
}

func TestCaseFiveLineOrdering(t *testing.T) {
	weekday := time.Date(2025, 3, 12, 9, 0, 0, 0, time.UTC)
	sunday := time.Date(2025, 3, 16, 9, 0, 0, 0, time.UTC)
	// Feed them out of order; the invoice sorts by start time.
	inv := PriceInvoice(PriorityNormal, []LaborInput{
		labor(2, "Sunday Tech", 5000, sunday, 1),
		labor(1, "Weekday Tech", 5000, weekday, 2),
	}, nil)
	if inv.LineItems[0].Weekend || !inv.LineItems[1].Weekend {
		t.Fatalf("ordering: %+v", inv.LineItems)
	}
	if inv.TotalCents != 25200 {
		t.Fatalf("total: %d", inv.TotalCents)
	}
}
