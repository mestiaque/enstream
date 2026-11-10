package server

import (
	"encoding/json"
	"net/http"
)

// Envelope is the shape of every JSON response.
type Envelope struct {
	Success    bool        `json:"success"`
	Data       any         `json:"data,omitempty"`
	Message    string      `json:"message,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Error      *APIError   `json:"error,omitempty"`
}

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func NewPagination(page, limit, total int) *Pagination {
	pages := 0
	if limit > 0 {
		pages = (total + limit - 1) / limit
	}
	return &Pagination{Page: page, Limit: limit, Total: total, TotalPages: pages}
}

// APIError is what clients see: a stable code and a readable message, never internal
// details (those go to the log with the request id).
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, data any) { WriteJSON(w, http.StatusOK, Envelope{Success: true, Data: data}) }

func OKPage(w http.ResponseWriter, data any, p *Pagination) {
	WriteJSON(w, http.StatusOK, Envelope{Success: true, Data: data, Pagination: p})
}

func Created(w http.ResponseWriter, data any, msg string) {
	WriteJSON(w, http.StatusCreated, Envelope{Success: true, Data: data, Message: msg})
}

func Fail(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, Envelope{Success: false, Error: &APIError{Code: code, Message: msg}})
}

// DecodeJSON reads a JSON body (size already limited by middleware), rejecting unknown fields.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
