# Contribute

Some note about the projects to get you started on contribution.

## CI/CD Pipeline

The project includes a complete CI/CD pipeline using GitHub Actions:

### Workflow Overview

1. **Lint and Format** - Runs on every push and PR
   - Validates Go code formatting with `gofmt`
   - Runs `golangci-lint` for code quality checks
   - Fails if code is not properly formatted or has linting issues

2. **Build and Test** - Runs after successful linting
   - Runs all tests with `go test ./...`
   - Builds the binary to ensure compilation works
   - Tests binary execution

3. **Docker Build and Push** - Runs only on `main` branch
   - Builds multi-platform Docker images (linux/amd64, linux/arm64)
   - Pushes images to GitHub Container Registry (GHCR)
   - Tags images with branch name, commit SHA, and `latest`

### Docker Support

The application can be run in a Docker container:

```bash
# Build Docker image locally
docker build -t tgifreezeday .

# Run with environment variables
docker run --rm \
  -e GOOGLE_APP_CLIENT_CRED_JSON_PATH=/app/creds.json \
  -e LOG_LEVEL=info \
  -e LOG_FORMAT=json \
  -v /path/to/creds.json:/app/creds.json \
  -v /path/to/config.yaml:/app/config.yaml \
  tgifreezeday sync
```

### GitHub Container Registry

Pre-built images are available at:
- `ghcr.io/nvat/tgifreezeday:latest` (latest main branch)
- `ghcr.io/nvat/tgifreezeday:main-<commit-sha>` (specific commit)

### Development Workflow

1. Make changes to code
2. Run `gofmt -s -w .` to format code
3. Run `make test` to ensure tests pass
4. Run `make build` to ensure binary builds
5. Push to branch - CI will run linting and tests
6. Create PR to main - full CI/CD pipeline runs
7. Merge to main - Docker image is built and pushed to GHCR

## Usage

### Build and Run

```bash
# Build the application
make build

# Run sync command (debug mode with colors)
make sync

# Run wipe-blockers command (debug mode with colors)
make wipe-blockers

# Or run manually
./bin/tgifreezeday sync
./bin/tgifreezeday wipe-blockers
./bin/tgifreezeday list-blockers
```

## Structure

The project follows Go best practices with clean architecture:

```
.
├── cmd/
│   └── tgifreezeday/        # Main application entry point
│       └── main.go          # CLI commands (sync, wipe-blockers, list-blockers)
├── internal/
│   ├── adapter/
│   │   └── googlecalendar/  # Google Calendar API implementation
│   ├── config/              # Configuration loading and validation
│   ├── consts/              # Constants (supported countries, etc.)
│   ├── domain/              # Core business logic and models
│   ├── helpers/             # Utility functions
│   └── logging/             # Structured logging setup
├── docs/                    # Documentation and images
├── .github/workflows/       # CI/CD pipeline (lint, test, build, deploy)
├── config.yaml              # Application configuration
├── Dockerfile               # Multi-platform container build
└── Makefile                 # Development commands
```

## Installation

- Requires Go 1.24+
- Clone repo, run `go build ./cmd/tgifreezeday`
- Set up Google API credentials (see below)
- Run binary with config file or env vars

## Configuration

- Set `GOOGLE_APP_CLIENT_CRED_JSON_PATH` to your service account JSON
  - For ReadFrom: need to enable Calendar API in your Google Project
  - For WriteTo: need to add the permissions of service account handler to your target calendar.

- Set `LOG_LEVEL` to control logging verbosity (debug, info, warn, error, fatal, panic). Default: info
- Set `LOG_FORMAT` to control log output format (json, text, colored). Default: json
- Optionally set `CONFIG_PATH` to your YAML config file
- See example config in README above

### Log Levels

The application uses structured logging with the following levels:
- `debug` - Detailed information for debugging
- `info` - General information about application flow (default)
- `warn` - Warning messages
- `error` - Error messages
- `fatal` - Fatal errors that cause the application to exit
- `panic` - Panic level (causes panic)

### Log Formats

The application supports different log output formats:
- `json` - Structured JSON format (default, good for log aggregation)
- `text` or `keyvalue` - Key-value text format (human-readable)
- `colored` or `color` - Colored key-value format (good for development)

Examples:
```bash
# JSON format (default)
LOG_LEVEL=debug ./bin/tgifreezeday sync

# Key-value format for human reading
LOG_FORMAT=text LOG_LEVEL=info ./bin/tgifreezeday sync

# Colored format for development
LOG_FORMAT=colored LOG_LEVEL=debug ./bin/tgifreezeday sync
```

## Contribution

PRs welcome. Add tests for new features. Follow idiomatic Go. Open issues for bugs/requests.