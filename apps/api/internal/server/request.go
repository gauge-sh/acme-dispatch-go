package server

import (
	"encoding/json"
	"mime"
	"net/http"
	"strconv"

	"github.com/acme-dispatch/dispatch/core"

	"github.com/acme-dispatch/dispatch/core/store"
)

// Opt distinguishes an absent PATCH key from an explicit null: absent leaves
// the field alone, null clears a nullable one.
type Opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (o *Opt[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// apply writes a required field; explicit null is rejected by the caller
// checking Null first via requireSet.
func (o Opt[T]) apply(dst *T) {
	if o.Set && !o.Null {
		*dst = o.Value
	}
}

func (o Opt[T]) applyPtr(dst **T) {
	if !o.Set {
		return
	}
	if o.Null {
		*dst = nil
		return
	}
	value := o.Value
	*dst = &value
}

// nullOn collects required fields that were explicitly nulled.
func nullOn(fields map[string]string, name string, set, null bool) {
	if set && null {
		fields[name] = "must not be null"
	}
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return core.Invalid("invalid request body")
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		media, _, err := mime.ParseMediaType(ct)
		if err != nil || media != "application/json" {
			return core.Invalid("requests must be application/json")
		}
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return core.Invalid("invalid request body")
	}
	return nil
}

func pathID(r *http.Request) (int64, bool) {
	return pathInt(r, "id")
}

func pathInt(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

type pageParams struct {
	Number int
	Size   int
}

func (p pageParams) store() store.Page {
	return store.Page{Number: p.Number, Size: p.Size}
}

func parsePage(r *http.Request) (pageParams, error) {
	page := pageParams{Number: 1, Size: 20}
	q := r.URL.Query()
	bad := core.Invalid("page and per_page must be positive integers")
	if raw := q.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return page, bad
		}
		page.Number = n
	}
	if raw := q.Get("per_page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return page, bad
		}
		if n > 100 {
			n = 100 // clamped, not an error
		}
		page.Size = n
	}
	return page, nil
}

func queryInt64(r *http.Request, name string) (*int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, core.Invalid(name + " must be an integer")
	}
	return &n, nil
}

func queryBool(r *http.Request, name string) (*bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	switch raw {
	case "true":
		v := true
		return &v, nil
	case "false":
		v := false
		return &v, nil
	}
	return nil, core.Invalid(name + " must be true or false")
}
