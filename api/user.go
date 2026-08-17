// GET /user/:slug  (rewritten from /api/user?slug=...  — see vercel.json)
package api

import (
	"database/sql"
	"net/http"

	"github.com/jacobwechuli/musicstory/pkg/db"
	"github.com/jacobwechuli/musicstory/pkg/render"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	database, err := db.Open()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	page, err := db.GetUserWithSongs(r.Context(), database, slug)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := render.User(w, page); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
