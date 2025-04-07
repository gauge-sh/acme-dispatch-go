package core

import (
	"fmt"
	"slices"
	"time"
)

// The pricing constants as exact ratios: labor markup ×1.6, weekend
// surcharge +15%, urgent surcharge +10%.
const (
	laborMarkupNum, laborMarkupDen           = 16, 10
	weekendSurchargeNum, weekendSurchargeDen = 23, 20
	urgentSurchargeNum, urgentSurchargeDen   = 11, 10
)

type LaborInput struct {
	AppointmentID   int64
	TechnicianName  string
	HourlyCostCents int64
	Start           time.Time
	End             time.Time
}

func (l LaborInput) seconds() int64 {
	return int64(l.End.Sub(l.Start) / time.Second)
}

func (l LaborInput) weekend() bool {
	return IsWeekend(l.Start)
}

// laborAmountCents rounds after the base line and again after the weekend
// surcharge — the surcharge lands on the rounded line, not the raw product,
// so the two roundings show up as two separate steps.
func laborAmountCents(l LaborInput) int64 {
	base := divRound(l.seconds()*l.HourlyCostCents*laborMarkupNum, 3600*laborMarkupDen)
	if !l.weekend() {
		return base
	}
	return divRound(base*weekendSurchargeNum, weekendSurchargeDen)
}

type PartInput struct {
	JobPartID      int64
	SKU            string
	Name           string
	Quantity       int64
	UnitPriceCents int64
}

type PricedInvoice struct {
	LaborCents int64
	PartsCents int64
	TotalCents int64
	LineItems  []LineItem
}

// FormatHours renders a duration the way it reads on an invoice: 2, 1.5,
// 0.25, 0.33. Two decimals is the finest granularity worth printing; the
// label never drives an amount, which always uses the exact duration.
func FormatHours(d time.Duration) string {
	hundredths := divRound(int64(d/time.Second)*100, 3600)
	whole, rem := hundredths/100, hundredths%100
	switch {
	case rem == 0:
		return fmt.Sprintf("%d", whole)
	case rem%10 == 0:
		return fmt.Sprintf("%d.%d", whole, rem/10)
	default:
		return fmt.Sprintf("%d.%02d", whole, rem)
	}
}

func laborLine(l LaborInput) LineItem {
	duration := l.End.Sub(l.Start)
	day := l.Start.UTC().Format("2006-01-02")
	return LineItem{
		Kind:        LineLabor,
		Description: fmt.Sprintf("%s — %sh on %s", l.TechnicianName, FormatHours(duration), day),
		Quantity:    float64(l.seconds()) / 3600,
		UnitCents:   l.HourlyCostCents,
		AmountCents: laborAmountCents(l),
		Weekend:     l.weekend(),
	}
}

func partLine(p PartInput) LineItem {
	return LineItem{
		Kind:        LinePart,
		Description: fmt.Sprintf("%s — %s", p.SKU, p.Name),
		Quantity:    float64(p.Quantity),
		UnitCents:   p.UnitPriceCents,
		// Whole cents times a whole quantity: nothing to round.
		AmountCents: p.Quantity * p.UnitPriceCents,
		Weekend:     false,
	}
}

// PriceInvoice prices a job's appointments and parts. Labor lines come first,
// ordered by scheduled start then appointment id; part lines follow in
// job-part order. The urgent multiplier is the last step and applies to the
// total alone — line amounts stay as billed.
func PriceInvoice(priority Priority, labor []LaborInput, parts []PartInput) PricedInvoice {
	labor = slices.Clone(labor)
	slices.SortStableFunc(labor, func(a, b LaborInput) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}
		return int(a.AppointmentID - b.AppointmentID)
	})
	parts = slices.Clone(parts)
	slices.SortStableFunc(parts, func(a, b PartInput) int {
		return int(a.JobPartID - b.JobPartID)
	})

	invoice := PricedInvoice{LineItems: make([]LineItem, 0, len(labor)+len(parts))}
	for _, l := range labor {
		line := laborLine(l)
		invoice.LaborCents += line.AmountCents
		invoice.LineItems = append(invoice.LineItems, line)
	}
	for _, p := range parts {
		line := partLine(p)
		invoice.PartsCents += line.AmountCents
		invoice.LineItems = append(invoice.LineItems, line)
	}
	invoice.TotalCents = invoice.LaborCents + invoice.PartsCents
	if priority == PriorityUrgent {
		invoice.TotalCents = divRound(invoice.TotalCents*urgentSurchargeNum, urgentSurchargeDen)
	}
	return invoice
}

// CheckInvoiceable gates invoice creation. A job that already has a record
// breaks both rules at once; the duplicate wins so the caller hears about
// the record (400) instead of the status (409).
func CheckInvoiceable(status JobStatus, alreadyInvoiced bool) error {
	if alreadyInvoiced {
		return Invalid("job is already invoiced")
	}
	if status != StatusCompleted {
		return InvalidTransition("only a completed job can be invoiced")
	}
	return nil
}
