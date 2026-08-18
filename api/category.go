// POST /api/category — create a new category for an existing user profile.
//
// Expected request body:
//
//	{
//	  "user_slug":    "kevin",
//	  "name":         "All-time favorites",
//	  "description":  "optional intro text shown at the top of the category page",
//	  "songs": [
//	    { "input": "https://open.spotify.com/track/...", "note": "" },
//	    { "input": "Anti Hero Taylor Swift", "note": "my note" }
//	  ]
//	}
//
// On success returns {"slug": "all-time-favorites", "user_slug": "kevin"}.
package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacobwechuli/musicstory/pkg/captioner"
	"github.com/jacobwechuli/musicstory/pkg/db"
	"github.com/jacobwechuli/musicstory/pkg/models"
	"github.com/jacobwechuli/musicstory/pkg/spotify"
)

var catSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyCat(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = catSlugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type categorySong struct {
	Input string `json:"input"`
	Note  string `json:"note"`
}

type categoryRequest struct {
	UserSlug    string         `json:"user_slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Songs       []categorySong `json:"songs"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.UserSlug = strings.TrimSpace(req.UserSlug)
	req.Name = strings.TrimSpace(req.Name)
	if req.UserSlug == "" {
		http.Error(w, "user_slug is required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(req.Songs) == 0 {
		http.Error(w, "at least one song is required", http.StatusBadRequest)
		return
	}

	database, err := db.Open()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	// Resolve every song to a Spotify type + ID.
	songs := make([]models.Song, 0, len(req.Songs))
	type pendingCaption struct {
		index  int
		title  string
		artist string
	}
	var needsCaption []pendingCaption

	for i, in := range req.Songs {
		s := models.Song{Position: i + 1, Caption: strings.TrimSpace(in.Note)}

		if parsed, ok := spotify.ParseURL(in.Input); ok {
			s.SpotifyType = parsed.Type
			s.SpotifyID = parsed.ID
		} else {
			result, err := spotify.SearchTrack(clientID, clientSecret, in.Input)
			if err != nil {
				http.Error(w, "couldn't find a match for: "+in.Input, http.StatusUnprocessableEntity)
				return
			}
			s.SpotifyType = result.Type
			s.SpotifyID = result.ID
		}
		songs = append(songs, s)

		if s.Caption == "" {
			title, artist, _ := spotify.GetTrackMeta(clientID, clientSecret, s.SpotifyID)
			needsCaption = append(needsCaption, pendingCaption{index: i, title: title, artist: artist})
		}
	}

	// Batch-generate missing captions.
	if len(needsCaption) > 0 {
		inputs := make([]captioner.SongInput, len(needsCaption))
		for i, p := range needsCaption {
			inputs[i] = captioner.SongInput{Position: p.index, Title: p.title, Artist: p.artist}
		}
		results, err := captioner.GenerateCaptions(r.Context(), req.UserSlug, inputs)
		if err == nil {
			for _, res := range results {
				songs[res.Position].Caption = res.Caption
				songs[res.Position].AutoCaption = true
			}
		}
	}

	// Derive a slug from the category name and de-duplicate within this user.
	catSlug := slugifyCat(req.Name)
	for i := 0; ; i++ {
		candidate := catSlug
		if i > 0 {
			candidate = catSlug + "-" + strconv.Itoa(i+1)
		}
		exists, err := db.CategorySlugExists(r.Context(), database, req.UserSlug, candidate)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if !exists {
			catSlug = candidate
			break
		}
	}

	if err := db.CreateCategory(r.Context(), database, req.UserSlug, catSlug, req.Name, req.Description, songs); err != nil {
		http.Error(w, "could not save category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"slug":      catSlug,
		"user_slug": req.UserSlug,
	})
}
