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

#### Config Format:

The program will accept the following config to run.

```yaml
readFrom:
  googleCalendar:
    id: <google calendary id to read>
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

#### Example:

If your organization has the following rules:

> As a general rule, production operations should **not** be conducted on the following days:
> - **First business day of the month**
> - **Last business day of the month**
> - **The day before public holidays or non-work day**

The folloiwng config should reflect the above rule:

```yaml
readFrom:
  googleCalendar:
    id: <google calendar id to read from>
    todayIsFreezeDayIf:
      today:
      - isTheFirstBusinessDayOfTheMonth
      today:
      - isTheLastBusinessDayOfTheMonth
      tomorrow:
      - isNonBusinessday
writeTo:
  googleCalendar:
    id: <google calendary id to read>
    ifTodayIsFreezeDay:
      default:
        summary: "Today is FREEZE-DAY. no PROD operation is allowed."
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
- Optionally set `CONFIG_PATH` to your YAML config file
- See example config in README above

## License

MIT. Free to use, modify, distribute, but must retain source and attribution.

## Contribution

PRs welcome. Add tests for new features. Follow idiomatic Go. Open issues for bugs/requests.