package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

// errorBody is the standardized error envelope: {"error": {code, message}}.
type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}

// writeStoreError maps common store errors onto HTTP responses.
func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", err.Error())
}

// decodeJSON strictly decodes a request body into dst.
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
