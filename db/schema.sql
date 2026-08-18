-- Run this once against your Railway Postgres instance.

CREATE TABLE users (
  id            SERIAL PRIMARY KEY,
  slug          TEXT UNIQUE NOT NULL,        -- becomes the URL: /user/{slug}
  display_name  TEXT NOT NULL,
  tagline       TEXT,                        -- optional one-liner under their name
  story         TEXT,                        -- a couple sentences about them + music
  auto_story    BOOLEAN NOT NULL DEFAULT false, -- true if the LLM wrote it
  created_at    TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE songs (
  id            SERIAL PRIMARY KEY,
  user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  position      INT NOT NULL,                -- display order, 1-based
  spotify_type  TEXT NOT NULL,                -- 'track' | 'album' | 'playlist'
  spotify_id    TEXT NOT NULL,                -- the ID pulled from the Spotify URL
  caption       TEXT,                         -- written by the user OR generated
  auto_caption  BOOLEAN NOT NULL DEFAULT false, -- true if the LLM wrote it
  created_at    TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_songs_user_position ON songs(user_id, position);

-- Categories: named groups of songs a user creates (e.g. "All-time favorites")
CREATE TABLE categories (
  id            SERIAL PRIMARY KEY,
  user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slug          TEXT NOT NULL,               -- URL segment: /user/{user_slug}/category/{cat_slug}
  name          TEXT NOT NULL,               -- display name, e.g. "All-time favorites"
  description   TEXT,                        -- optional intro lede shown at top of the category page
  position      INT NOT NULL DEFAULT 0,      -- display order on the profile page
  created_at    TIMESTAMPTZ DEFAULT now(),
  UNIQUE (user_id, slug)
);

CREATE INDEX idx_categories_user ON categories(user_id, position);

-- Category songs: which songs belong to a category (songs can be in multiple categories)
CREATE TABLE category_songs (
  id            SERIAL PRIMARY KEY,
  category_id   INT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  position      INT NOT NULL,
  spotify_type  TEXT NOT NULL,
  spotify_id    TEXT NOT NULL,
  caption       TEXT,
  auto_caption  BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_category_songs_category ON category_songs(category_id, position);
