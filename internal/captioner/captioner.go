// Package captioner — fills in missing song captions using the Groq API.
// Call this once per submission, AFTER resolving Spotify IDs and BEFORE
// saving to the database — batch every uncaptioned song from one submission
// into a single request instead of one call per song.
package captioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type SongInput struct {
	Position int    `json:"position"`
	Title    string `json:"title"`  // track/album name, from Spotify lookup
	Artist   string `json:"artist"` // from Spotify lookup
	Note     string `json:"note"`   // whatever the user typed, if anything — pass through as context
}

type captionResult struct {
	Position int    `json:"position"`
	Caption  string `json:"caption"`
}

// GenerateCaptions asks Claude for one short caption per song, in the same
// voice already used across the rest of the site. Only call this for songs
// where the user didn't write their own note — check for that BEFORE
// calling, since skipping the call entirely is both cheaper and more
// authentic than overwriting what they already said.
func GenerateCaptions(ctx context.Context, displayName string, songs []SongInput) ([]captionResult, error) {
	if len(songs) == 0 {
		return nil, nil
	}

	systemPrompt := `You write short, personal, one-line captions for songs on a ` +
		`friend's music page. Match this voice: warm, a little wry, plainspoken — ` +
		`like "the one I put on for long drives," not like ad copy. Under 15 words each. ` +
		`Return ONLY a JSON array of {"position": <int>, "caption": "<text>"}, nothing else.`

	userPrompt, err := json.Marshal(map[string]any{
		"display_name": displayName,
		"songs":        songs,
	})
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]any{
		"model":      "llama-3.1-8b-instant", // cheap + fast, this task doesn't need more
		"max_tokens": 500,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": string(userPrompt)},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq api returned %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	var text string
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}

	var results []captionResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return nil, fmt.Errorf("could not parse caption response: %w", err)
	}
	return results, nil
}
