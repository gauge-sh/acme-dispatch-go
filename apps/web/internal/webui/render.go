package webui

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/acme-dispatch/dispatch/core"
)

//go:embed templates static
var assets embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

var funcs = template.FuncMap{
	"money": func(cents int64) string {
		sign := ""
		if cents < 0 {
			sign, cents = "-", -cents
		}
		return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
	},
	"datetime": func(t core.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.UTC().Format("Jan 2 2006, 15:04")
	},
	"date": func(t core.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.UTC().Format("2006-01-02")
	},
	"inputTime": func(t core.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02T15:04")
	},
	"title": func(s string) string {
		s = strings.ReplaceAll(s, "_", " ")
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"join": func(items []string, sep string) string {
		return strings.Join(items, sep)
	},
	"list": func(items ...string) []string {
		return items
	},
	"mul": func(a, b int64) int64 {
		return a * b
	},
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"hours": core.FormatHours,
}

// pages maps a page name to its parsed template set: layout plus the page.
var pages = func() map[string]*template.Template {
	entries, err := fs.Glob(assets, "templates/pages/*.tmpl")
	if err != nil {
		panic(err)
	}
	parsed := make(map[string]*template.Template, len(entries))
	for _, entry := range entries {
		name := strings.TrimSuffix(strings.TrimPrefix(entry, "templates/pages/"), ".tmpl")
		parsed[name] = template.Must(
			template.New("layout.tmpl").Funcs(funcs).ParseFS(assets, "templates/layout.tmpl", entry))
	}
	return parsed
}()

// view is the envelope every page renders inside.
type view struct {
	User  *core.User
	Flash string
	Data  any
}

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	tmpl, ok := pages[page]
	if !ok {
		log.Printf("unknown template %q", page)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	v := view{Data: data}
	if user := currentUser(r); user != nil {
		v.User = user
	}
	v.Flash = a.popFlash(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "layout.tmpl", v); err != nil {
		log.Printf("render %s: %v", page, err)
	}
}

func (a *App) serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// formError extracts what a re-rendered form needs from a failed save.
func formError(err error) (message string, fields map[string]string, ok bool) {
	if derr, isDomain := err.(*core.Error); isDomain {
		fields = derr.Fields
		if fields == nil {
			fields = map[string]string{}
		}
		return derr.Message, fields, true
	}
	return "", nil, false
}
