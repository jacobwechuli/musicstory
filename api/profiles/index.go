// GET /profiles
package handler

import (
	"net/http"

	"github.com/jacobwechuli/musicstory/pkg/db"
	"github.com/jacobwechuli/musicstory/pkg/render"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	database, err := db.Open()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	page, err := db.GetAllProfiles(r.Context(), database)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := render.Profiles(w, page); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
