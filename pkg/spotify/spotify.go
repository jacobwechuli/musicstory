// Package spotify resolves whatever a user gives us — a pasted link or a
// typed song name — into a usable Spotify type + ID, and can fetch basic
// metadata (title/artist) for captioning context.
package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ParsedLink is what we get back once we know what a user gave us.
type ParsedLink struct {
	Type string // "track" | "album" | "playlist"
	ID   string
}

var linkPattern = regexp.MustCompile(`open\.spotify\.com/(?:embed/)?(track|album|playlist)/([a-zA-Z0-9]+)`)

// ParseURL extracts type + ID from any Spotify share or embed URL. Returns
// ok=false if the input doesn't look like a Spotify URL at all — callers
// should treat that as "this is a plain song name, go search for it instead."
func ParseURL(input string) (ParsedLink, bool) {
	matches := linkPattern.FindStringSubmatch(input)
	if len(matches) != 3 {
		return ParsedLink{}, false
	}
	return ParsedLink{Type: matches[1], ID: matches[2]}, true
}

// --- Client Credentials auth (app-level, no user login needed) ---

var (
	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time
)

func getAccessToken(clientID, clientSecret string) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if cachedToken != "" && time.Now().Before(tokenExpiry) {
		return cachedToken, nil
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("spotify token request returned %d", resp.StatusCode)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}

	cachedToken = parsed.AccessToken
	tokenExpiry = time.Now().Add(time.Duration(parsed.ExpiresIn-30) * time.Second) // refresh a little early
	return cachedToken, nil
}

// --- Search (for plain song names) ---

type SearchResult struct {
	Type   string
	ID     string
	Title  string
	Artist string
}

// searchTracks is the shared implementation used by both SearchTrack and
// SearchTracks — it returns up to `limit` results for a query.
func searchTracks(clientID, clientSecret, query string, limit int) ([]SearchResult, error) {
	token, err := getAccessToken(clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	endpoint := "https://api.spotify.com/v1/search?" + url.Values{
		"q":     {query},
		"type":  {"track"},
		"limit": {fmt.Sprintf("%d", limit)},
	}.Encode()

	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify search returned %d", resp.StatusCode)
	}

	var parsed struct {
		Tracks struct {
			Items []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"items"`
		} `json:"tracks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(parsed.Tracks.Items))
	for _, item := range parsed.Tracks.Items {
		artist := ""
		if len(item.Artists) > 0 {
			artist = item.Artists[0].Name
		}
		results = append(results, SearchResult{Type: "track", ID: item.ID, Title: item.Name, Artist: artist})
	}
	return results, nil
}

// SearchTrack finds the best-guess match for a typed song name. Returns the
// top result only — callers building a confirm-before-save UI should show
// this back to the user rather than saving silently, since a name search
// can surface the wrong version (remaster, deluxe edition, etc).
func SearchTrack(clientID, clientSecret, query string) (SearchResult, error) {
	results, err := searchTracks(clientID, clientSecret, query, 1)
	if err != nil {
		return SearchResult{}, err
	}
	if len(results) == 0 {
		return SearchResult{}, fmt.Errorf("no results for %q", query)
	}
	return results[0], nil
}

// SearchTracks returns up to 5 results for a typed query — used by the
// autocomplete endpoint so the user can pick the right version.
func SearchTracks(clientID, clientSecret, query string) ([]SearchResult, error) {
	results, err := searchTracks(clientID, clientSecret, query, 5)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for %q", query)
	}
	return results, nil
}

// GetTrackMeta fetches title/artist for a track we already have an ID for
// (e.g. resolved from a pasted link) — used to give the captioner real
// context instead of just a bare Spotify ID.
func GetTrackMeta(clientID, clientSecret, trackID string) (title, artist string, err error) {
	token, err := getAccessToken(clientID, clientSecret)
	if err != nil {
		return "", "", err
	}

	req, _ := http.NewRequest("GET", "https://api.spotify.com/v1/tracks/"+trackID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("spotify track lookup returned %d", resp.StatusCode)
	}

	var parsed struct {
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", "", err
	}
	if len(parsed.Artists) > 0 {
		artist = parsed.Artists[0].Name
	}
	return parsed.Name, artist, nil
}
