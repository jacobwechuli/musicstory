// Package db owns the Postgres connection and every query. Nothing outside
// this package should write raw SQL — api/ and internal/render/ call these
// functions and get back plain models.* structs.
package db

import (
	"context"
	"database/sql"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

	"yourmodule/internal/models"
)

var (
	pool     *sql.DB
	poolOnce sync.Once
	poolErr  error
)

// Open returns a shared, lazily-initialized connection pool. Safe to call
// from every handler invocation — Vercel may reuse a warm function instance
// across requests, and this avoids opening a fresh connection every time.
func Open() (*sql.DB, error) {
	poolOnce.Do(func() {
		pool, poolErr = sql.Open("pgx", os.Getenv("DATABASE_URL"))
	})
	return pool, poolErr
}

// GetUserWithSongs loads one profile and its songs, ordered for display.
// Returns sql.ErrNoRows if the slug doesn't exist — callers should turn
// that into a 404.
func GetUserWithSongs(ctx context.Context, database *sql.DB, slug string) (models.UserPage, error) {
	var page models.UserPage

	err := database.QueryRowContext(ctx,
		`SELECT slug, display_name, COALESCE(tagline, ''), COALESCE(story, '')
		 FROM users WHERE slug = $1`,
		slug,
	).Scan(&page.Slug, &page.DisplayName, &page.Tagline, &page.Story)
	if err != nil {
		return page, err
	}

	rows, err := database.QueryContext(ctx,
		`SELECT position, spotify_type, spotify_id, COALESCE(caption, '')
		 FROM songs WHERE user_id = (SELECT id FROM users WHERE slug = $1)
		 ORDER BY position ASC`,
		slug,
	)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	for rows.Next() {
		var s models.Song
		if err := rows.Scan(&s.Position, &s.SpotifyType, &s.SpotifyID, &s.Caption); err != nil {
			return page, err
		}
		page.Songs = append(page.Songs, s)
	}

	return page, rows.Err()
}

// GetAllProfiles loads every user for the /profiles directory, oldest first.
func GetAllProfiles(ctx context.Context, database *sql.DB) (models.ProfilesPage, error) {
	var page models.ProfilesPage

	rows, err := database.QueryContext(ctx,
		`SELECT slug, display_name, COALESCE(tagline, '') FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.ProfileSummary
		if err := rows.Scan(&u.Slug, &u.DisplayName, &u.Tagline); err != nil {
			return page, err
		}
		page.Users = append(page.Users, u)
	}

	return page, rows.Err()
}

// SlugExists checks availability before creating a new profile.
func SlugExists(ctx context.Context, database *sql.DB, slug string) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE slug = $1)`, slug,
	).Scan(&exists)
	return exists, err
}

// CreateUserWithSongs inserts a new profile and its songs in one transaction.
// Songs should already have final captions resolved (written by the user,
// or filled in by the captioner) before this is called.
func CreateUserWithSongs(ctx context.Context, database *sql.DB, slug, displayName, tagline, story string, autoStory bool, songs []models.Song) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (slug, display_name, tagline, story, auto_story)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		slug, displayName, nullIfEmpty(tagline), nullIfEmpty(story), autoStory,
	).Scan(&userID)
	if err != nil {
		return err
	}

	for _, s := range songs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO songs (user_id, position, spotify_type, spotify_id, caption, auto_caption)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			userID, s.Position, s.SpotifyType, s.SpotifyID, nullIfEmpty(s.Caption), s.AutoCaption,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
