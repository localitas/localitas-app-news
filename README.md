# News

RSS/Atom feed reader and article manager.

Part of the [Localitas](https://github.com/localitas) platform — a self-hosted, privacy-first personal computing system.

## Features

- Subscribe to any RSS or Atom feed
- Automatic background sync
- Article tagging and favorites
- Full-text search across articles
- Default feeds seeded on first launch

## Installation

### Development (via Localitas core)

```bash
# Clone the repo
git clone https://github.com/localitas/localitas-app-news.git ~/localitas-app-news

# Start with the Localitas dev cluster (builds and runs in Docker automatically)
cd ~/localitas && make dev-core
```

### Standalone

```bash
cd ~/localitas-app-news

# Build and run locally
make build
./bin/news-server serve --listen :8000

# Or via launchd (macOS)
make start

# Or via Docker
make start-docker
```

## Exposing to the Internet

Localitas apps are accessible remotely through Localitas's built-in tunnel service, powered by FRP. No port forwarding or dynamic DNS required.

1. Sign up at [localitas.com](https://localitas.com) and connect your local Localitas core
2. The tunnel automatically exposes your core (and all apps) at `https://{your-subdomain}.localitas.com`
3. This app is available at `https://{your-subdomain}.localitas.com/apps/ext/news/`

All traffic is encrypted end-to-end. Authentication is handled by the Localitas core — only authorized users can access your apps.

## License

MIT
