// GET /profiles  (rewritten from /api/profiles — see vercel.json)
package api

import (
	"net/http"

	"yourmodule/internal/db"
	"yourmodule/internal/render"
)

func ProfilesHandler(w http.ResponseWriter, r *http.Request) {
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
