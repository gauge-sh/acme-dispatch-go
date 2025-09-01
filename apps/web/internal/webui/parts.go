package webui

import (
	"fmt"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type partFormPage struct {
	Part   *core.Part
	Cost   string
	Price  string
	Error  string
	Fields map[string]string
}

func partForm(p *core.Part) partFormPage {
	page := partFormPage{Part: p}
	if p.UnitCostCents > 0 {
		page.Cost = fmt.Sprintf("%d.%02d", p.UnitCostCents/100, p.UnitCostCents%100)
	}
	if p.UnitPriceCents > 0 {
		page.Price = fmt.Sprintf("%d.%02d", p.UnitPriceCents/100, p.UnitPriceCents%100)
	}
	return page
}

func (a *App) partList(w http.ResponseWriter, r *http.Request) {
	page := listPage(r)
	query := formStr(r, "q")
	// The stock join keeps quantities on the same screen as the catalog.
	levels, total, err := a.store.ListStockLevels(store.StockFilter{}, page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if query != "" {
		parts, partTotal, err := a.store.ListParts(store.PartFilter{Query: query}, page)
		if err != nil {
			a.serverError(w, err)
			return
		}
		filtered := make([]store.StockWithPart, 0, len(parts))
		for _, part := range parts {
			level, err := a.store.StockLevelForPart(part.ID)
			if err != nil {
				a.serverError(w, err)
				return
			}
			filtered = append(filtered, store.StockWithPart{StockLevel: *level, Part: part})
		}
		levels, total = filtered, partTotal
	}
	a.render(w, r, http.StatusOK, "parts", struct {
		Levels []store.StockWithPart
		Query  string
		Pager  pager
	}{levels, query, makePager(r, page, total)})
}

func (a *App) partNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, http.StatusOK, "part_form", partForm(&core.Part{}))
}

func partFromForm(r *http.Request, p *core.Part) error {
	fields := map[string]string{}
	p.SKU = formStr(r, "sku")
	p.Name = formStr(r, "name")
	p.UnitCostCents = formCents(r, "unit_cost", fields)
	p.UnitPriceCents = formCents(r, "unit_price", fields)
	return fieldsToError("cost and price must be dollar amounts", fields)
}

func (a *App) renderPartForm(w http.ResponseWriter, r *http.Request, part *core.Part, message string, fields map[string]string) {
	page := partForm(part)
	page.Cost = formStr(r, "unit_cost")
	page.Price = formStr(r, "unit_price")
	page.Error, page.Fields = message, fields
	a.render(w, r, http.StatusUnprocessableEntity, "part_form", page)
}

func (a *App) partCreate(w http.ResponseWriter, r *http.Request) {
	var part core.Part
	err := partFromForm(r, &part)
	if err == nil {
		err = a.store.CreatePart(&part)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			a.renderPartForm(w, r, &part, message, fields)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Part created with an empty stock level.")
	http.Redirect(w, r, fmt.Sprintf("/parts/%d", part.ID), http.StatusSeeOther)
}

func (a *App) partDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	part, err := a.store.PartByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	level, err := a.store.StockLevelForPart(id)
	if err != nil {
		a.serverError(w, err)
		return
	}
	adjustments, _, err := a.store.ListStockAdjustments(&id, store.Page{Number: 1, Size: 50})
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, http.StatusOK, "part_detail", struct {
		Part        *core.Part
		Level       *core.StockLevel
		Adjustments []store.StockAdjustment
	}{part, level, adjustments})
}

func (a *App) partEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	part, err := a.store.PartByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, http.StatusOK, "part_form", partForm(part))
}

func (a *App) partUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	part, err := a.store.PartByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = partFromForm(r, part)
	if err == nil {
		err = a.store.UpdatePart(part)
	}
	if err != nil {
		if message, fields, ok := formError(err); ok {
			a.renderPartForm(w, r, part, message, fields)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Part updated.")
	http.Redirect(w, r, fmt.Sprintf("/parts/%d", id), http.StatusSeeOther)
}

func (a *App) partDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.DeletePart(id); err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/parts/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Part deleted.")
	http.Redirect(w, r, "/parts", http.StatusSeeOther)
}

func (a *App) stockAdjust(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	fields := map[string]string{}
	delta := formInt(r, "delta", fields)
	reason := formOpt(r, "reason")
	var err error
	if err = fieldsToError("delta must be a whole number", fields); err == nil {
		_, err = a.store.AdjustStockLevel(id, delta, reason)
	}
	if err != nil {
		if message, _, ok := formError(err); ok {
			a.setFlash(w, r, message)
			http.Redirect(w, r, fmt.Sprintf("/parts/%d", id), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	a.setFlash(w, r, "Stock adjusted.")
	http.Redirect(w, r, fmt.Sprintf("/parts/%d", id), http.StatusSeeOther)
}
