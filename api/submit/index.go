// POST /api/submit
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
	"github.com/jacobwechuli/musicstory/pkg/storyteller"
)

type submitSong struct {
	Input string `json:"input"`
	Note  string `json:"note"`
}

type submitRequest struct {
	DisplayName string       `json:"display_name"`
	Tagline     string       `json:"tagline"`
	Story       string       `json:"story"`
	WantAIStory bool         `json:"want_ai_story"`
	StoryNotes  string       `json:"story_notes"`
	Songs       []submitSong `json:"songs"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
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

	if len(needsCaption) > 0 {
		inputs := make([]captioner.SongInput, len(needsCaption))
		for i, p := range needsCaption {
			inputs[i] = captioner.SongInput{Position: p.index, Title: p.title, Artist: p.artist}
		}
		results, err := captioner.GenerateCaptions(r.Context(), req.DisplayName, inputs)
		if err == nil {
			for _, res := range results {
				songs[res.Position].Caption = res.Caption
				songs[res.Position].AutoCaption = true
			}
		}
	}

	story := strings.TrimSpace(req.Story)
	autoStory := false
	if story == "" && req.WantAIStory {
		generated, err := storyteller.GenerateStory(r.Context(), req.DisplayName, req.StoryNotes)
		if err == nil {
			story = generated
			autoStory = true
		}
	}

	slug := slugify(req.DisplayName)
	for i := 0; ; i++ {
		candidate := slug
		if i > 0 {
			candidate = slug + "-" + strconv.Itoa(i+1)
		}
		exists, err := db.SlugExists(r.Context(), database, candidate)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if !exists {
			slug = candidate
			break
		}
	}

	if err := db.CreateUserWithSongs(r.Context(), database, slug, req.DisplayName, req.Tagline, story, autoStory, songs); err != nil {
		http.Error(w, "could not save profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"slug": slug})
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
