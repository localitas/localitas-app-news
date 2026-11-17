---
title: News
description: RSS feed reader and article manager
---

# News

Subscribe to RSS feeds, organize them into collections, and read articles.

Each user has their own subscriptions, collections, tags, and read and
starred articles. Two users who subscribe to the same feed share its fetched
articles, never each other's reading state. What the household had before
reading became per-user goes to the first admin who opens News.

## Collections

Group feeds into named collections for organization.

**GET /api/collections** - List all collections
**POST /api/collections** - Create a new collection
**DELETE /api/collections/{id}** - Delete a collection

## Feeds

Add RSS/Atom feeds and sync them to pull new articles.

**GET /api/feeds** - List all subscribed feeds
**POST /api/feeds** - Subscribe to a new feed
**DELETE /api/feeds/{id}** - Unsubscribe from a feed
**POST /api/feeds/sync** - Refresh feeds and pull new articles

## Articles

Browse and manage synced articles. Mark articles as read, star favorites, and tag them for organization.

**GET /api/articles** - List articles (filter by feed, collection, read status)
**POST /api/articles/{id}/read** - Mark an article as read
**POST /api/articles/{id}/star** - Toggle star on an article

## Tags

Create tags and apply them to articles for custom categorization.

**GET /api/tags** - List all tags
**POST /api/tags** - Create a new tag
**DELETE /api/tags/{id}** - Delete a tag
**POST /api/articles/{id}/tag** - Tag an article
**DELETE /api/articles/{id}/tags/{tag_id}** - Remove a tag from an article
**GET /api/tags/{id}/articles** - List articles with a specific tag

## Search

**GET /api/search** - Full-text search across article titles and content

## Default Feeds

**GET /api/defaults** - List built-in default feed suggestions to get started quickly

## Build & Deploy

Run these in the app's repository, `~/localitas-app-news`.

### Version

```bash
bin/news-server --version
```

### Build from source

```bash
# Native binary (bin/news-server)
make build

# Linux binary the Docker image is built from, for the Docker daemon's
# architecture (news-server-linux-arm64 on Apple silicon); LINUX_ARCH=amd64 overrides
make build-linux
```

Both run `make lint` (gofmt and go vet) first; `make test` runs the lint step and then the tests.

### Run natively

```bash
make start   # builds bin/news-server, writes the launchd service com.localitas.app.news for bin/news-server and loads it on port 9204
make stop
make logs    # follows ~/.localitas/logs/news/stdout.log
```

The app announces itself to core when it starts, and core serves it at `/apps/ext/news/`.

### Docker

```bash
make build-docker   # builds the Linux binary and the news:latest image
make start-docker   # runs container "news" on port 9204, labelled localitas.app=true
make stop-docker
make logs-docker
```

Core registers containers labelled `localitas.app=true` once Docker reports them healthy.

### Publish

```bash
# Run the tests, build the image, push ghcr.io/localitas/localitas-app-news:latest and :<VERSION>
make docker-push

# Build dist/news-server-darwin-arm64 and dist/news-server-darwin-amd64
# and create GitHub release vX.Y.Z on this repository with them
make release VERSION=vX.Y.Z
```

`VERSION` defaults to `git describe --tags --always --dirty`.

### Download

Pre-built macOS binaries are on the [releases page](https://github.com/localitas/localitas-app-news/releases):
- `news-server-darwin-arm64` (Apple Silicon)
- `news-server-darwin-amd64` (Intel)

Every push to `main` publishes a release: `.github/workflows/release.yml` runs the tests and releases both binaries as `v0.1.<commit count>.<UTC timestamp>`. `make release` publishes one by hand.

Download with the GitHub CLI:

    gh release download --repo localitas/localitas-app-news --pattern 'news-server-*'
