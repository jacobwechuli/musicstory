// GET /user/:slug/category/:cat  (rewritten via vercel.json)
// Renders a single category page for a user.
package api

import (
	"database/sql"
	"net/http"

	"github.com/jacobwechuli/musicstory/pkg/db"
	"github.com/jacobwechuli/musicstory/pkg/render"
)

func CategoryPageHandler(w http.ResponseWriter, r *http.Request) {
	userSlug := r.URL.Query().Get("slug")
	catSlug := r.URL.Query().Get("cat")
	if userSlug == "" || catSlug == "" {
		http.NotFound(w, r)
		return
	}

	database, err := db.Open()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	cat, err := db.GetCategoryWithSongs(r.Context(), database, userSlug, catSlug)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := render.Category(w, userSlug, cat); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
