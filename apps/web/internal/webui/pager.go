package webui

import (
	"net/http"
	"strconv"

	"github.com/acme-dispatch/dispatch/core/store"
)

type pager struct {
	Page    int
	Pages   int
	Total   int
	PrevURL string
	NextURL string
}

func listPage(r *http.Request) store.Page {
	page := store.Page{Number: 1, Size: 20}
	if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 0 {
		page.Number = n
	}
	return page
}

func makePager(r *http.Request, page store.Page, total int) pager {
	pages := (total + page.Size - 1) / page.Size
	if pages < 1 {
		pages = 1
	}
	p := pager{Page: page.Number, Pages: pages, Total: total}
	link := func(n int) string {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(n))
		return r.URL.Path + "?" + q.Encode()
	}
	if page.Number > 1 {
		p.PrevURL = link(page.Number - 1)
	}
	if page.Number < pages {
		p.NextURL = link(page.Number + 1)
	}
	return p
}
