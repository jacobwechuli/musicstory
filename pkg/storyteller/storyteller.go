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
)

// Notes is whatever the user optionally jotted down — a sentence or two on
// what music means to them, a favorite memory, anything. Can be empty; if
// it is, the story comes out generic and short rather than invented.
func GenerateStory(ctx context.Context, displayName string, notes string) (string, error) {
	systemPrompt := `You write a short personal blurb (2-3 sentences, no more) for someone's ` +
		`music page, in this voice: plainspoken, warm, a little wry — never overwrought or ` +
		`poetic-for-its-own-sake. Keep it simple. If notes are given, stay close to what they ` +
		`actually said — do not invent specific memories, people, or details they didn't mention. ` +
		`If no notes are given, write something short and honest, like "no big story here — ` +
		`just a few songs worth sharing," in a way that still sounds like a real sentence, not ` +
		`a placeholder. Return ONLY the story text. No quotes, no preamble, no markdown.`

	userContent := fmt.Sprintf("Name: %s\nNotes: %s", displayName, notes)
	if notes == "" {
		userContent = fmt.Sprintf("Name: %s\nNotes: (none given)", displayName)
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
