package webui

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/acme-dispatch/dispatch/core"
)

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func formStr(r *http.Request, name string) string {
	return strings.TrimSpace(r.FormValue(name))
}

// formOpt turns an empty input into nil so blank form fields clear nullable
// columns instead of storing "".
func formOpt(r *http.Request, name string) *string {
	value := formStr(r, name)
	if value == "" {
		return nil
	}
	return &value
}

func formInt(r *http.Request, name string, fields map[string]string) int64 {
	raw := formStr(r, name)
	if raw == "" {
		fields[name] = "is required"
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fields[name] = "must be a whole number"
		return 0
	}
	return n
}

// formCents reads a dollars-and-cents input like "12.99" into cents.
func formCents(r *http.Request, name string, fields map[string]string) int64 {
	raw := formStr(r, name)
	if raw == "" {
		fields[name] = "is required"
		return 0
	}
	negative := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	whole, frac, _ := strings.Cut(raw, ".")
	if whole == "" {
		whole = "0"
	}
	switch len(frac) {
	case 0:
		frac = "00"
	case 1:
		frac += "0"
	case 2:
	default:
		fields[name] = "must be a dollar amount like 12.99"
		return 0
	}
	dollars, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		fields[name] = "must be a dollar amount like 12.99"
		return 0
	}
	cents, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		fields[name] = "must be a dollar amount like 12.99"
		return 0
	}
	total := dollars*100 + cents
	if negative {
		total = -total
	}
	return total
}

// formTime reads a datetime-local input; the office schedules in UTC.
func formTime(r *http.Request, name string, fields map[string]string) core.Time {
	raw := formStr(r, name)
	if raw == "" {
		fields[name] = "is required"
		return core.Time{}
	}
	parsed, err := time.Parse("2006-01-02T15:04", raw)
	if err != nil {
		fields[name] = "must be a date and time"
		return core.Time{}
	}
	return core.NewTime(parsed)
}

func formTags(r *http.Request, name string) []string {
	raw := formStr(r, name)
	if raw == "" {
		return nil
	}
	var tags []string
	for _, tag := range strings.Split(raw, ",") {
		tags = append(tags, strings.TrimSpace(tag))
	}
	return tags
}

func fieldsToError(message string, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	return core.InvalidFields(message, fields)
}
