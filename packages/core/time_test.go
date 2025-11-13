package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeJSONRoundTrip(t *testing.T) {
	moment := NewTime(time.Date(2025, 3, 12, 13, 0, 0, 0, time.UTC))
	raw, err := json.Marshal(moment)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"2025-03-12T13:00:00Z"` {
		t.Fatalf("wire format: %s", raw)
	}
	var back Time
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Equal(moment.Time) {
		t.Fatalf("round trip drifted: %v vs %v", back, moment)
	}
}

func TestTimeZeroIsNull(t *testing.T) {
	raw, err := json.Marshal(Time{})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "null" {
		t.Fatalf("zero time: %s", raw)
	}
	var back Time
	if err := json.Unmarshal([]byte("null"), &back); err != nil {
		t.Fatal(err)
	}
	if !back.IsZero() {
		t.Fatal("null should scan to the zero time")
	}
}

func TestTimeNormalizesToUTCSeconds(t *testing.T) {
	offset := time.FixedZone("plus5", 5*3600)
	local := time.Date(2025, 3, 12, 18, 0, 0, 999_000_000, offset)
	moment := NewTime(local)
	if moment.String() != "2025-03-12T13:00:00Z" {
		t.Fatalf("normalization: %s", moment)
	}
}

func TestTimeScanFormats(t *testing.T) {
	var moment Time
	if err := moment.Scan("2025-03-15T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if !IsWeekend(moment.Time) {
		t.Fatal("scan lost the value")
	}
	if err := moment.Scan(nil); err != nil || !moment.IsZero() {
		t.Fatalf("nil scan: %v %v", moment, err)
	}
	if err := moment.Scan("not a time"); err == nil {
		t.Fatal("garbage accepted")
	}
	if err := moment.Scan(42); err == nil {
		t.Fatal("int accepted")
	}
}

func TestStringListRoundTrip(t *testing.T) {
	value, err := StringList{"hvac", "plumbing"}.Value()
	if err != nil {
		t.Fatal(err)
	}
	var back StringList
	if err := back.Scan(value); err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 || back[0] != "hvac" {
		t.Fatalf("round trip: %v", back)
	}

	var empty StringList
	if err := empty.Scan(nil); err != nil || empty == nil {
		t.Fatalf("nil scan should give an empty list: %v %v", empty, err)
	}

	raw, _ := json.Marshal(StringList(nil))
	if string(raw) != "[]" {
		t.Fatalf("nil list must serialize as []: %s", raw)
	}
}
