# TGI Freeze Day

A Go application that helps announce production freeze days to ensure safe deployments by:

1. Fetching holidays and special events from Google Calendar
2. Updating a specific calendar with freeze days information
3. Sending notifications to Slack channels

## Features

- Automatic holiday detection from Google Calendar
- Configurable rules for determining freeze days (e.g., day before/after holidays)
- Slack notifications for upcoming freeze days
- Support for weekend freeze periods
- Calendar integration to maintain an up-to-date freeze day calendar

## Architecture

The application follows clean architecture principles to ensure separation of concerns and maintainability:

- **Core Domain**: Contains the business entities and rules independent of external frameworks
- **Ports**: Define interfaces that the core domain expects from external adapters
- **Adapters**: Implement the interfaces defined by ports, connecting to external systems
- **Services**: Implement use cases by orchestrating domain objects and ports

### Project Structure

```
├── cmd/                # Application entry points
│   └── tgifreezeday/   # Main application command
├── internal/           # Private application code
│   ├── core/           # Core business logic
│   │   ├── domain/     # Domain models
│   │   ├── ports/      # Interfaces defining the boundaries
│   │   └── services/   # Use cases implementation
│   ├── adapters/       # Implementations of the interfaces
│   │   ├── datasource/ # Data source adapters (Google Calendar, etc.)
│   │   └── destination/# Destination adapters (Google Calendar, etc.)
│   └── config/         # Configuration parsing and validation
└── pkg/                # Public libraries that can be used by external applications
    └── rules/          # Rule engine and expression evaluation
```

## Installation

```bash
# Clone the repository
git clone https://github.com/nvat/tgifreezeday.git
cd tgifreezeday

# Build the application
go build -o tgifreezeday ./cmd/tgifreezeday

# Run the application
./tgifreezeday
```

## Configuration

Copy the example configuration file and modify it for your environment:

```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your settings
```

The configuration file has two main sections:

1. **data_source**: Defines the holiday calendar source (currently Google Calendar) and contains the rules for determining freeze days
2. **destination**: Defines where freeze periods should be created/announced (Google Calendar or Slack)

Example:

```yaml
# Data source configuration
data_source:
  type: google_calendar
  config:
    credentials_file: "/path/to/your/google/credentials.json"
    calendar_id: "your_calendar_id_with_holidays@group.calendar.google.com"
  rules:
    - expression: "today.isHoliday()"
    - expression: "tomorrow.isHoliday()"
    - expression: "today.isWeekend()"

# Destination configuration
destination:
  type: google_calendar
  config:
    credentials_file: "/path/to/your/google/credentials.json"
    calendar_id: "your_primary_calendar@gmail.com"
```

### Setting up Google Calendar API

1. Create a project in Google Cloud Platform
2. Enable the Google Calendar API
3. Create a service account with appropriate permissions
4. Download the credentials JSON file
5. Share your calendars with the service account email

## Usage

```bash
# Run with default configuration (looks for config.yaml)
./tgifreezeday

# Specify a custom config file
./tgifreezeday --config=custom-config.yaml

# Check if today is a freeze day
./tgifreezeday --check-today

# List upcoming freeze days
./tgifreezeday --list-upcoming

# List upcoming freeze days for next 14 days
./tgifreezeday --list-upcoming --days=14
```

## Development

### Running with Docker

You can use Docker to run the application:

```bash
# Build the Docker image
docker build -t tgifreezeday .

# Run the container with your configuration
docker run -v /path/to/your/config.yaml:/app/config.yaml tgifreezeday
```

You can also use the provided docker-compose.yml:

```bash
docker-compose up
```

### Testing

Run the tests with:

```bash
go test ./...
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
