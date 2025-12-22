package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/acme-dispatch/dispatch/core"
)

var statusForCode = map[core.ErrorCode]int{
	core.CodeValidationFailed:  http.StatusBadRequest,
	core.CodeNotFound:          http.StatusNotFound,
	core.CodeInvalidTransition: http.StatusConflict,
	core.CodeScheduleConflict:  http.StatusConflict,
	core.CodeInsufficientStock: http.StatusConflict,
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]errorBody{"error": {Code: code, Message: message}})
}

// writeError maps a domain error onto the wire; anything else is a 500 that
// deliberately says nothing about what happened.
func writeError(w http.ResponseWriter, err error) {
	var derr *core.Error
	if errors.As(err, &derr) {
		body := errorBody{Code: string(derr.Code), Message: derr.Message}
		if len(derr.Fields) > 0 {
			body.Fields = derr.Fields
		}
		writeJSON(w, statusForCode[derr.Code], map[string]errorBody{"error": body})
		return
	}
	log.Printf("internal error: %v", err)
	writeErrorCode(w, http.StatusInternalServerError, "internal", "something went wrong")
}

func notFound(w http.ResponseWriter, entity string) {
	writeErrorCode(w, http.StatusNotFound, "not_found", entity+" not found")
}

type listEnvelope struct {
	Items   any `json:"items"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

func writeList(w http.ResponseWriter, items any, page pageParams, total int) {
	writeJSON(w, http.StatusOK, listEnvelope{Items: items, Page: page.Number, PerPage: page.Size, Total: total})
}
