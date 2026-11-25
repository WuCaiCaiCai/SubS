package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// decodeJSON reads a size-capped JSON body.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, max int64) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, max)).Decode(dst)
}

// pathID parses the {id} route parameter.
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
