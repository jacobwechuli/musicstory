// Package models holds the shared data shapes used across the backend —
// db reads them out of Postgres, render fills them into templates, submit
// builds them from an incoming form. One definition, no duplication.
package models

type Song struct {
	Position    int
	SpotifyType string // "track" | "album" | "playlist"
	SpotifyID   string
	Caption     string
	AutoCaption bool // true if the LLM wrote this caption, false if the user did
}

type UserPage struct {
	Slug        string
	DisplayName string
	Tagline     string
	Story       string
	Songs       []Song
}

type ProfileSummary struct {
	Slug        string
	DisplayName string
	Tagline     string
}

type ProfilesPage struct {
	Users []ProfileSummary
}
