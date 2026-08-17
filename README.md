# Ted's music site — backend

## Structure

```
public/           static pages, served as-is by Vercel
api/               one file = one serverless function
  user.go            GET  /user/:slug     → renders a profile
  profiles.go        GET  /profiles       → renders the directory
  submit.go          POST /api/submit     → intake form target
internal/
  models/            shared structs — the only place types are defined
  db/                Postgres connection + every query
  render/            the two Go templates + execution helpers
  captioner/         batched, only for songs missing a caption
  storyteller/        one call, only if the user opts into AI writing their story
  spotify/           URL parsing, search, track metadata lookup
db/schema.sql       run this once against Railway
```

## Before you deploy

1. **Rename the module.** Everything currently imports `yourmodule/internal/...`.
   Run `go mod init github.com/<you>/<repo>` (or whatever path you want) and
   find/replace `yourmodule` across every `.go` file to match.
2. **Run `db/schema.sql`** against your Railway Postgres instance.
3. **Set env vars** in Vercel's project settings:
   - `DATABASE_URL` — from Railway
   - `ANTHROPIC_API_KEY`
   - `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` — from Spotify's developer dashboard
4. **Test locally** with `vercel dev` before pushing — Vercel's Go runtime
   conventions occasionally shift, so confirm each `api/*.go` file is being
   picked up as its own function before you rely on it in production.

## How a request flows

```
Browser → GET /user/kevin
        → vercel.json rewrite → /api/user?slug=kevin
        → api/user.go
        → internal/db (fetch)
        → internal/render (execute template)
        → HTML back to browser
```

```
Browser → POST /api/submit  (from create-profile.html's form)
        → internal/spotify   (resolve each song → type + ID)
        → internal/captioner (ONE batched call, only for songs with no note)
        → internal/storyteller (ONE call, only if they asked for AI help)
        → internal/db        (insert user + songs, transactional)
        → { "slug": "kevin" } back to the browser
        → browser redirects itself to /user/kevin
```

## What's genuinely done vs. still rough

**Done and functional:**
- Every route (`/user/:slug`, `/profiles`, `/api/submit`)
- Spotify link parsing AND name search, both paths
- Batched captioning, optional AI story
- Slug collision handling (adds `-2`, `-3`, etc.)
- The actual intake form (`create-profile.html`), wired end to end

**Worth hardening before this is public, even to just 3 people:**
- No input length limits — someone pasting a novel into a caption field
  will happily get stored as-is
- No retry logic if the Anthropic or Spotify calls fail mid-request — right
  now a failure just means that song/story silently has no caption, which
  is a reasonable fallback but you may want to surface it to the user instead
- `SearchTrack` returns only the top result — worth a confirm-before-save
  step in the form if wrong-version matches turn out to be a problem
