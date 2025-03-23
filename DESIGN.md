# Design

## Rough Overview

This program reads from a data source, iterates through a list of rule sets, and evaluates each rule. If a rule evaluates to true, the program will proceed to one of its destinations and perform an action. There are three main concepts: data sources, rule sets, and destinations.

Firstly, a data source is an origin from which the program gets data. For example, Google Calendar can be a data source, providing dates, events, and metadata about those dates.

Secondly, rule sets consist of lists of rules that the program evaluates. For example, a rule might check if a date is a holiday. If all rules fail, the program does nothing.

Finally, destinations are where actions are performed based on rule evaluation. Currently, Google Calendar is the primary destination. Based on the rule set evaluation, the program will perform actions on the destination. In our use case, it creates an event that blocks all free space in your calendar.

In summary, a data source will have multiple rule sets, and there will be one destination per instance.

### Data Source

Currently, the program only supports Google Calendar as a data source. It retrieves dates, events, and metadata about those dates.

### Rule Set

Rule sets depend on the data source object. In the configuration, rule sets are nested within the data source object. Rule sets are arrays of items containing expressions, such as "today is holiday" or "tomorrow is holiday". When these expressions evaluate to true, they trigger an action to a destination.

### Destination

Currently, only Google Calendar is supported as a destination. The action performed is creating a "busy" event that starts at 8am local time and ends at 6pm local time, with the name "Production Release Forbidden".

## Example Config

```yaml
data_source:
  type: google_calendar
  config:
    calendar_id: "your_calendar_id" # the Google official calendar ID for public holidays

  rules:
  - expression: "today.isHoliday()"
  - expression: "tomorrow.isHoliday()"
  - expression: "today.isWeekend()"

destination:
  type: google_calendar
  config:
    calendar_id: "your_calendar_id"
```

## Project Structure

This project is written in Go and follows a clean architecture pattern to maximize extensibility. The structure consists of:

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

This structure separates concerns, making it easier to extend the application with new data sources or destinations without modifying existing code. The core business logic is isolated from external dependencies through interfaces (ports) that are implemented by adapters.