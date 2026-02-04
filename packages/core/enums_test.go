package core

import "testing"

func TestParseEnums(t *testing.T) {
	if _, ok := ParseRole("admin"); !ok {
		t.Error("admin should parse")
	}
	for _, bad := range []string{"", "Admin", "superuser"} {
		if _, ok := ParseRole(bad); ok {
			t.Errorf("role %q parsed", bad)
		}
	}

	for _, good := range []string{"low", "normal", "urgent"} {
		if _, ok := ParsePriority(good); !ok {
			t.Errorf("priority %q should parse", good)
		}
	}
	if _, ok := ParsePriority("URGENT"); ok {
		t.Error("wire values are lower-case only")
	}

	for _, st := range JobStatuses {
		if parsed, ok := ParseJobStatus(string(st)); !ok || parsed != st {
			t.Errorf("status %q should round-trip", st)
		}
	}
	if _, ok := ParseJobStatus("in progress"); ok {
		t.Error("space is not an underscore")
	}
}

func TestNextStatusesHideInvoiced(t *testing.T) {
	// invoiced is reachable only through invoicing, so it never shows up as
	// an offered move — not even from completed.
	for _, from := range JobStatuses {
		for _, next := range NextStatuses(from) {
			if next == StatusInvoiced {
				t.Fatalf("NextStatuses(%s) offers invoiced", from)
			}
			if !CanTransition(from, next) {
				t.Fatalf("NextStatuses(%s) offers illegal %s", from, next)
			}
		}
	}
	if len(NextStatuses(StatusCompleted)) != 0 {
		t.Error("completed has no transition-endpoint moves")
	}
	if len(NextStatuses(StatusRequested)) != 2 {
		t.Errorf("requested: %v", NextStatuses(StatusRequested))
	}
}
