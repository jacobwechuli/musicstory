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
