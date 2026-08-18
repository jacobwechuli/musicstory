// Package db owns the Postgres connection and every query. Nothing outside
// this package should write raw SQL — api/ and pkg/render/ call these
// functions and get back plain models.* structs.
package db

import (
	"context"
	"database/sql"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jacobwechuli/musicstory/pkg/models"
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

// GetUserWithSongs loads one profile, its songs, and its categories (with
// their songs), all ordered for display.
// Returns sql.ErrNoRows if the slug doesn't exist — callers should turn
// that into a 404.
func GetUserWithSongs(ctx context.Context, database *sql.DB, slug string) (models.UserPage, error) {
	var page models.UserPage
	var userID int

	err := database.QueryRowContext(ctx,
		`SELECT id, slug, display_name, COALESCE(tagline, ''), COALESCE(story, '')
		 FROM users WHERE slug = $1`,
		slug,
	).Scan(&userID, &page.Slug, &page.DisplayName, &page.Tagline, &page.Story)
	if err != nil {
		return page, err
	}

	// Load the user's flat song list.
	rows, err := database.QueryContext(ctx,
		`SELECT position, spotify_type, spotify_id, COALESCE(caption, '')
		 FROM songs WHERE user_id = $1
		 ORDER BY position ASC`,
		userID,
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
	if err := rows.Err(); err != nil {
		return page, err
	}

	// Load the user's categories.
	catRows, err := database.QueryContext(ctx,
		`SELECT id, slug, name, COALESCE(description, ''), position
		 FROM categories WHERE user_id = $1
		 ORDER BY position ASC`,
		userID,
	)
	if err != nil {
		return page, err
	}
	defer catRows.Close()

	type catMeta struct {
		id  int
		cat models.Category
	}
	var cats []catMeta
	for catRows.Next() {
		var cm catMeta
		if err := catRows.Scan(&cm.id, &cm.cat.Slug, &cm.cat.Name, &cm.cat.Description, &cm.cat.Position); err != nil {
			return page, err
		}
		cats = append(cats, cm)
	}
	if err := catRows.Err(); err != nil {
		return page, err
	}

	// For each category load its songs.
	for _, cm := range cats {
		sRows, err := database.QueryContext(ctx,
			`SELECT position, spotify_type, spotify_id, COALESCE(caption, '')
			 FROM category_songs WHERE category_id = $1
			 ORDER BY position ASC`,
			cm.id,
		)
		if err != nil {
			return page, err
		}
		for sRows.Next() {
			var s models.Song
			if err := sRows.Scan(&s.Position, &s.SpotifyType, &s.SpotifyID, &s.Caption); err != nil {
				sRows.Close()
				return page, err
			}
			cm.cat.Songs = append(cm.cat.Songs, s)
		}
		sRows.Close()
		if err := sRows.Err(); err != nil {
			return page, err
		}
		page.Categories = append(page.Categories, cm.cat)
	}

	return page, nil
}

// GetCategoryWithSongs loads a single category (by user slug + category slug)
// with all its songs. Returns sql.ErrNoRows if either slug doesn't exist.
func GetCategoryWithSongs(ctx context.Context, database *sql.DB, userSlug, catSlug string) (models.Category, error) {
	var cat models.Category
	var catID int

	err := database.QueryRowContext(ctx,
		`SELECT c.id, c.slug, c.name, COALESCE(c.description, ''), c.position
		 FROM categories c
		 JOIN users u ON u.id = c.user_id
		 WHERE u.slug = $1 AND c.slug = $2`,
		userSlug, catSlug,
	).Scan(&catID, &cat.Slug, &cat.Name, &cat.Description, &cat.Position)
	if err != nil {
		return cat, err
	}

	rows, err := database.QueryContext(ctx,
		`SELECT position, spotify_type, spotify_id, COALESCE(caption, '')
		 FROM category_songs WHERE category_id = $1
		 ORDER BY position ASC`,
		catID,
	)
	if err != nil {
		return cat, err
	}
	defer rows.Close()
	for rows.Next() {
		var s models.Song
		if err := rows.Scan(&s.Position, &s.SpotifyType, &s.SpotifyID, &s.Caption); err != nil {
			return cat, err
		}
		cat.Songs = append(cat.Songs, s)
	}
	return cat, rows.Err()
}

// CreateCategory inserts a new category (with its songs) for an existing user.
// The category slug is derived from the name and de-duplicated within the user's
// existing categories by the caller.
func CreateCategory(ctx context.Context, database *sql.DB, userSlug, catSlug, name, description string, songs []models.Song) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get user ID and next position in one shot.
	var userID int
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE slug = $1`, userSlug).Scan(&userID)
	if err != nil {
		return err
	}

	var nextPos int
	_ = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM categories WHERE user_id = $1`, userID,
	).Scan(&nextPos)

	var catID int
	err = tx.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, slug, name, description, position)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID, catSlug, name, nullIfEmpty(description), nextPos,
	).Scan(&catID)
	if err != nil {
		return err
	}

	for _, s := range songs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO category_songs (category_id, position, spotify_type, spotify_id, caption, auto_caption)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			catID, s.Position, s.SpotifyType, s.SpotifyID, nullIfEmpty(s.Caption), s.AutoCaption,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CategorySlugExists checks whether a category slug is already taken for a given user.
func CategorySlugExists(ctx context.Context, database *sql.DB, userSlug, catSlug string) (bool, error) {
	var exists bool
	err := database.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM categories c
			JOIN users u ON u.id = c.user_id
			WHERE u.slug = $1 AND c.slug = $2
		)`, userSlug, catSlug,
	).Scan(&exists)
	return exists, err
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

// DeleteUser removes a user and all their data (songs, categories, category_songs)
// by cascade. Returns sql.ErrNoRows if the slug doesn't exist.
func DeleteUser(ctx context.Context, database *sql.DB, slug string) error {
	res, err := database.ExecContext(ctx, `DELETE FROM users WHERE slug = $1`, slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
