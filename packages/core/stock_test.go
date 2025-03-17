package core

import (
	"errors"
	"testing"
)

func TestAdjustStock(t *testing.T) {
	if got, err := AdjustStock(5, 3); err != nil || got != 8 {
		t.Fatalf("AdjustStock(5, 3) = %d, %v", got, err)
	}
	if got, err := AdjustStock(5, -5); err != nil || got != 0 {
		t.Fatalf("draining to exactly zero is allowed: %d, %v", got, err)
	}

	var derr *Error
	_, err := AdjustStock(5, -6)
	if !errors.As(err, &derr) || derr.Code != CodeInsufficientStock {
		t.Fatalf("expected insufficient_stock, got %v", err)
	}
	if derr.Message != "not enough stock on hand" {
		t.Fatalf("unexpected message %q", derr.Message)
	}

	_, err = AdjustStock(5, 0)
	if !errors.As(err, &derr) || derr.Code != CodeValidationFailed || derr.Message != "delta must not be zero" {
		t.Fatalf("zero delta: %v", err)
	}
}

func TestConsumeAndRelease(t *testing.T) {
	if got, err := ConsumeStock(10, 4); err != nil || got != 6 {
		t.Fatalf("ConsumeStock(10, 4) = %d, %v", got, err)
	}
	if got, err := ReleaseStock(6, 4); err != nil || got != 10 {
		t.Fatalf("ReleaseStock(6, 4) = %d, %v", got, err)
	}

	var derr *Error
	_, err := ConsumeStock(3, 4)
	if !errors.As(err, &derr) || derr.Code != CodeInsufficientStock {
		t.Fatalf("overdraw: %v", err)
	}
	for _, q := range []int64{0, -2} {
		_, err := ConsumeStock(10, q)
		if !errors.As(err, &derr) || derr.Message != "quantity must be greater than zero" {
			t.Errorf("ConsumeStock(10, %d): %v", q, err)
		}
		_, err = ReleaseStock(10, q)
		if !errors.As(err, &derr) || derr.Code != CodeValidationFailed {
			t.Errorf("ReleaseStock(10, %d): %v", q, err)
		}
	}
}
