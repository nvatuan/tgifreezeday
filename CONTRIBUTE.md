# Contribute

Some note about the projects to get you started on contribution.

## Structure

The project follows Go best practices with clean architecture:

```
.
├── cmd/
│   └── server/              # Main application entry point (HTTP server)
│       └── main.go          # Server setup, routing, env var validation
├── internal/
│   ├── adapter/
│   │   ├── db/              # SQLite persistence (users, OAuth tokens, configs)
│   │   └── googlecalendar/  # Google Calendar API implementation
│   ├── config/              # Config YAML loading and validation
│   ├── consts/              # Constants (supported countries, etc.)
│   ├── domain/              # Core business logic and models
│   ├── helpers/             # Utility functions
│   ├── logging/             # Structured logging setup
│   ├── perm/                # Role-based access control (Power/Write/ReadOnly)
│   ├── session/             # HTTP session management (signed cookies)
│   └── web/
│       └── handler/         # HTTP handlers (auth, dashboard, config, schema)
├── k8s/                     # Kubernetes manifests (StatefulSet, Service, Ingress, PVC)
├── docs/                    # Documentation and images
├── .github/workflows/       # CI/CD pipeline (lint, test, build, deploy)
├── .env.example             # Example environment variable file
├── config.yaml              # Example application config
├── Dockerfile               # Multi-platform container build
└── Makefile                 # Development commands
```

## Setup

### Prerequisites

- Go 1.25+
- Google Calendar API access
- OAuth 2.0 Client ID credentials (**Web application** type) from Google Cloud Console

### Configuration

#### Google OAuth credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/) → APIs & Services → Credentials
2. Create an **OAuth 2.0 Client ID** with application type **Web application**
3. Add your redirect URI to **Authorized redirect URIs** (e.g. `http://localhost:8080/oauth/callback`)
4. Enable the **Google Calendar API** in your project
5. Copy the Client ID and Client Secret from the credentials page

#### Environment variables

Copy `.env.example` to `.env` and fill in the values:

```bash
# Required
GOOGLE_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_OAUTH_CLIENT_SECRET=your-client-secret
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/callback
SESSION_SECRET=replace-with-32-plus-random-chars

# Optional (defaults shown)
PORT=8080
DB_PATH=./tgifreezeday.db
LOG_LEVEL=info
LOG_FORMAT=json
HTTPS_ONLY=false   # set true in production to add Secure flag to cookies
BASE_PATH=         # set to sub-path prefix if behind a reverse proxy (e.g. /tgifreezeday)

# Access control — comma-separated email lists
POWER_USER_EMAIL_LIST=admin@example.com           # full access: create/edit/delete any config
WRITE_USER_EMAIL_LIST=dev@example.com,ops@example.com  # create + manage own configs
                                                       # all other authenticated users: read-only
```

**Auth flow:** users log in via "Sign in with Google" in the browser. OAuth tokens and sessions are stored in the SQLite database (`DB_PATH`). There is no local token cache file.

#### Access control

| Role | Permissions |
|------|-------------|
| **Power** (`POWER_USER_EMAIL_LIST`) | Create, edit, delete, sync any config |
| **Write** (`WRITE_USER_EMAIL_LIST`) | Create configs; edit/delete/sync own configs |
| **Read-only** (everyone else) | View configs and blocker lists |

### Build and Run

```bash
# Build the server binary
make build

# Build and run in debug mode (colored logs)
make serve

# Or run manually
./bin/tgifreezeday
```

The server listens on `http://localhost:8080` by default. Open it in a browser to log in via Google OAuth.

### Log Levels

The application uses structured logging with the following levels:
- `debug` - Detailed information for debugging
- `info` - General information about application flow (default)
- `warn` - Warning messages
- `error` - Error messages
- `fatal` - Fatal errors that cause the application to exit

### Log Formats

The application supports different log output formats:
- `json` - Structured JSON format (default, good for log aggregation)
- `text` or `keyvalue` - Key-value text format (human-readable)
- `colored` or `color` - Colored key-value format (good for development)

Examples:
```bash
# JSON format (default)
LOG_LEVEL=debug ./bin/tgifreezeday

# Colored format for development (also what `make serve` uses)
LOG_FORMAT=colored LOG_LEVEL=debug ./bin/tgifreezeday
```

## CI/CD Pipeline

The project includes a complete CI/CD pipeline using GitHub Actions:

### Workflow Overview

1. **Lint and Format** - Runs on every push
   - Validates Go code formatting with `gofmt`
   - Runs `golangci-lint` for code quality checks
   - Fails if code is not properly formatted or has linting issues

2. **Build and Test** - Runs after successful linting
   - Runs all tests with `go test ./...`
   - Builds the server binary

3. **Docker Build and Push** - Runs only on `main` branch
   - Builds multi-platform Docker images (linux/amd64, linux/arm64)
   - Pushes images to GitHub Container Registry (GHCR)
   - Tags images with branch name, commit SHA, and `latest`

### Docker Support

The application runs as a long-lived web server. Pass all required env vars and mount a volume for the SQLite database:

```bash
# Build Docker image locally
docker build -t tgifreezeday .

# Run with environment variables
docker run --rm \
  -p 8080:8080 \
  -e GOOGLE_OAUTH_CLIENT_ID=your-client-id \
  -e GOOGLE_OAUTH_CLIENT_SECRET=your-secret \
  -e GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/oauth/callback \
  -e SESSION_SECRET=replace-with-32-plus-random-chars \
  -e LOG_LEVEL=info \
  -v /path/to/data:/data \
  -e DB_PATH=/data/tgifreezeday.db \
  tgifreezeday
```

### GitHub Container Registry

Pre-built images are available at:
- `ghcr.io/nvat/tgifreezeday:latest` (latest main branch)
- `ghcr.io/nvat/tgifreezeday:main-<commit-sha>` (specific commit)

### Kubernetes

The `k8s/` directory contains production-ready manifests (StatefulSet + PVC for SQLite, Service, Ingress, Vault-sourced secrets). The app runs as a single replica because SQLite is a single-writer database.

### Development Workflow

1. Make changes to code
2. Run `gofmt -s -w .` to format code
3. Run `make test` to ensure tests pass
4. Run `make build` to ensure binary builds
5. Push to branch - CI will run linting and tests
6. Create PR to main - full CI/CD pipeline runs
7. Merge to main - Docker image is built and pushed to GHCR

## Contribution

PRs welcome. Add tests for new features. Follow idiomatic Go. Open issues for bugs/requests.