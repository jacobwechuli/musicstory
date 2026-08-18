// DELETE /delete_profile?slug=:slug
package handler

import (
	"database/sql"
	"net/http"

	"github.com/jacobwechuli/musicstory/pkg/db"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Error(w, "slug required", http.StatusBadRequest)
		return
	}

	database, err := db.Open()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	err = db.DeleteUser(r.Context(), database, slug)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
