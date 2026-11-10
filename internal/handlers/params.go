package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"enstream/internal/server"
)

// pathID reads a positive integer URL parameter; writes a 400 and returns false otherwise.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		server.Fail(w, http.StatusBadRequest, "INVALID_ID", "The id must be a positive number")
		return 0, false
	}
	return id, true
}

var errBadParam = errors.New("bad parameter")

// intParam: optional integer query parameter within [min, max]; def when absent.
func intParam(r *http.Request, name string, def, min, max int) (int, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		return 0, errBadParam
	}
	return n, nil
}
