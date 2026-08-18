// Package storyteller — optionally writes the short "about them and music"
// blurb for a user's page. Only call this if the user chose NOT to write
// their own story — if they typed something themselves, save that directly
// and skip this entirely.
// Uses the Groq API.
package storyteller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GenerateStory writes a 2-3 sentence personality blurb driven by the genres
// the user selected. Notes are optional extra context the user typed.
func GenerateStory(ctx context.Context, displayName string, genres []string, notes string) (string, error) {
	systemPrompt := `You write a short personality story (2-3 sentences, no more) for someone's ` +
		`music page. You are given the music genres they identified with and optional personal notes. ` +
		`Use the genres to paint a picture of their musical personality — what kind of person listens ` +
		`to these genres, how they experience music, what it says about them. ` +
		`Voice: plainspoken, warm, a little wry — never overwrought or poetic-for-its-own-sake. ` +
		`If personal notes are given, weave them in naturally but do not invent details they didn't mention. ` +
		`Return ONLY the story text. No quotes, no preamble, no markdown.`

	genreList := strings.Join(genres, ", ")
	if genreList == "" {
		genreList = "(none given)"
	}
	userContent := fmt.Sprintf("Name: %s\nGenres: %s\nNotes: %s", displayName, genreList, notes)
	if notes == "" {
		userContent = fmt.Sprintf("Name: %s\nGenres: %s", displayName, genreList)
	}

	body, _ := json.Marshal(map[string]any{
		"model":      "llama-3.1-8b-instant",
		"max_tokens": 200,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq api returned %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}

	if len(parsed.Choices) > 0 {
		return parsed.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no content in groq response")
}
