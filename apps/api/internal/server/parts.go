package server

import (
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
	"github.com/acme-dispatch/dispatch/core/store"
)

type partPayload struct {
	SKU            Opt[string] `json:"sku"`
	Name           Opt[string] `json:"name"`
	UnitCostCents  Opt[int64]  `json:"unit_cost_cents"`
	UnitPriceCents Opt[int64]  `json:"unit_price_cents"`
}

func (p partPayload) checkNulls() error {
	fields := map[string]string{}
	nullOn(fields, "sku", p.SKU.Set, p.SKU.Null)
	nullOn(fields, "name", p.Name.Set, p.Name.Null)
	nullOn(fields, "unit_cost_cents", p.UnitCostCents.Set, p.UnitCostCents.Null)
	nullOn(fields, "unit_price_cents", p.UnitPriceCents.Set, p.UnitPriceCents.Null)
	return fieldError(fields)
}

func (p partPayload) applyTo(part *core.Part) {
	p.SKU.apply(&part.SKU)
	p.Name.apply(&part.Name)
	p.UnitCostCents.apply(&part.UnitCostCents)
	p.UnitPriceCents.apply(&part.UnitPriceCents)
}

func (s *Server) createPart(w http.ResponseWriter, r *http.Request) {
	var payload partPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	var part core.Part
	payload.applyTo(&part)
	if err := s.store.CreatePart(&part); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, part)
}

func (s *Server) getPart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "part not found")
		return
	}
	part, err := s.store.PartByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, part)
}

func (s *Server) patchPart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "part not found")
		return
	}
	part, err := s.store.PartByID(id)
	if err != nil {
		writeError(w, err)
		return
	}
	var payload partPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	if err := payload.checkNulls(); err != nil {
		writeError(w, err)
		return
	}
	payload.applyTo(part)
	if err := s.store.UpdatePart(part); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, part)
}

func (s *Server) deletePart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "part not found")
		return
	}
	if err := s.store.DeletePart(id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listParts(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter := store.PartFilter{Query: r.URL.Query().Get("q")}
	items, total, err := s.store.ListParts(filter, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}

func (s *Server) listStockLevels(w http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	partID, err := queryInt64(r, "part_id")
	if err != nil {
		writeError(w, err)
		return
	}
	items, total, err := s.store.ListStockLevels(store.StockFilter{PartID: partID}, page.store())
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, page, total)
}

func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request) {
	partID, ok := pathInt(r, "part_id")
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not_found", "part not found")
		return
	}
	var payload struct {
		Delta Opt[int64] `json:"delta"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, err)
		return
	}
	level, err := s.store.AdjustStockLevel(partID, payload.Delta.Value, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, level)
}
