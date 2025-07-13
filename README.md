# TGI Freeze Day

A Go application that helps announce production freeze days to ensure safe deployments by:

1. Fetching holidays and special events from Google Calendar
2. Updating a specific calendar with freeze days information

## Features

### v1:

#### Core Functionality

For version 1, I am for these following features only:
- Reads from a source calendar, checking if today is a freeze day. Freeze day in v1 is defined as:
  - If today is the first business day of the month
  - If today is a non-business day (eg. holiday, weekends, etc.)
  - If tomorrow is a non-business day (eg. holiday, weekends, etc.)
- If today is a freeze day, on a destination calendar, do this "default" behavior:
  - Create a blocking (eg. "busy") event that spans from 8AM to 8PM that said "Today is Freeze-day."
  - The "default" option can be passed with a "summary" that override the default message above.

- "business day" is defined as day that normally business is being conducted.
- Google Calendar is the only Calendar supported

#### Non-core functionality

- In case of a rule change after first run, the program must be able to remove old events and add new events that reflects the rules set. Because of that, some design effort must go to how to identify events that are created by the tool.
- The program is designed to run periodically (eg. daily, weekly,...) so a "window" should be specify so it can update forward. For instance, window=7d meaning it will check the next 7 days for freeze days and mark the calendar, to avoid running for so long.
- Program must check for permission issue and fail-fast if no permission to read or write.
- Program must fail-fast if not set necessary environment variables
- Should use client library instead of calling API HTTP requests.

## Commands

The application supports the following commands:

- `tgifreezeday sync` - Full sync: wipe existing blockers and create new ones based on freeze day rules
- `tgifreezeday wipe-blockers` - Remove all existing blockers in the specified time range

## Config Format:

The program will accept the following config to run.

```yaml
shared:
  lookbackDays: 20     # Days to look back from today (min: 20, max: 60)
  lookaheadDays: 60    # Days to look ahead from today (min: 20, max: 60)
readFrom:
  googleCalendar:
    countryCode: <supported country code> # "jpn", "vnm", A-3 ISO 3166 country code
    todayIsFreezeDayIf:
    - [yesterday, today, tomorrow]: # with this block, rules are AND together. To do OR, specify multiple items with same key.
      - isTheFirstBusinessDayOfTheMonth
      - isTheLastBusinessDayOfTheMonth
      - isNonBusinessDay
writeTo:
  googleCalendar:
    id: <google calendary id to read>
    ifTodayIsFreezeDay:
      default:
        summary: "string|null" # if `null`, use default message
```

#### Supported Countries:

- **jpn** - Japanese public holidays (filters out cultural observances like Tanabata)
- **vnm**

#### Example:

If your organization has the following rules:

> As a general rule, production operations should **not** be conducted on the following days:
> - **First business day of the month**
> - **Last business day of the month**
> - **The day before public holidays or non-work day**

The folloiwng config should reflect the above rule:

```yaml
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
      - today:
        - isTheLastBusinessDayOfTheMonth
      - tomorrow:
        - isNonBusinessDay
writeTo:
  googleCalendar:
    id: <google calendary id to read>
    ifTodayIsFreezeDay:
      default:
        summary: "Today is FREEZE-DAY. no PROD operation is allowed."
```

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
```

## Structure

Use golang. A simple but extensible directory structure should be consider. Future features should be:
- Allow Slack notification.
- Customize Google Calendar events
- OpenTelemetry

Should have a config package to read environment var and populate a struct there. Centralized the configs should allow easier on-boarding contribution from new contributors.

Should have proper unit-testing, and have mock for Google Calendar to allow testing.

```
tgifreezeday/
├── cmd/
│   └── tgifreezeday/
│       └── main.go              # Application entrypoint
├── internal/
│   ├── config/
│   │   └── config.go            # Config loading and validation
│   ├── calendar/
│   │   └── calendar.go          # Google Calendar logic, business day rules
│   ├── freeze/
│   │   └── freeze.go            # Freeze day calculation logic
│   ├── events/
│   │   └── events.go            # Event creation/deletion logic
│   └── mock/
│       └── mock_calendar.go     # Test mocks
├── pkg/                         # Reusable packages (if any)
├── go.mod
├── go.sum
└── README.md
```

## Installation

- Requires Go 1.20+
- Clone repo, run `go build ./cmd/tgifreezeday`
- Set up Google API credentials (see below)
- Run binary with config file or env vars

## Configuration

- Set `GOOGLE_APP_CLIENT_CRED_JSON_PATH` to your service account JSON
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

## License

MIT. Free to use, modify, distribute, but must retain source and attribution.

## Contribution

PRs welcome. Add tests for new features. Follow idiomatic Go. Open issues for bugs/requests.