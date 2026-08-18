// GET /api/search?q=<query>
// Autocomplete endpoint — returns up to 5 Spotify track results as JSON.
// Credentials stay server-side; the browser never sees them.
package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/jacobwechuli/musicstory/pkg/spotify"
)

type searchItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	URL    string `json:"url"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	results, err := spotify.SearchTracks(clientID, clientSecret, q)
	if err != nil {
		// Return empty list — autocomplete failure should be silent.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	items := make([]searchItem, len(results))
	for i, r := range results {
		items[i] = searchItem{
			ID:     r.ID,
			Type:   r.Type,
			Title:  r.Title,
			Artist: r.Artist,
			URL:    "https://open.spotify.com/" + r.Type + "/" + r.ID,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
