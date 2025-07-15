# TGI Freeze Day

<img src="./docs/tgifreezeday.png">

A Go application that helps announce production freeze days to ensure safe deployments by:

1. Fetching holidays events from Google Calendar
2. Updating a specific calendar with freeze days information based on what you defined as a freeze-day

## Concepts

- **Freeze Day**: A day when production deployments are restricted to reduce risk
- **Blocker Event**: A calendar event (8AM-8PM) that signals "no deployments allowed" on the calendar. This serves as a notice for everyone.

## How It Works

```mermaid
sequenceDiagram
    participant App as tgifreezeday
    participant Source as Holiday Calendar
    participant Target as Team Calendar

    App->>Source: Read public holidays
    App->>App: Calculate freeze days
    App->>Target: Create blocker events
```

1. **Reads** public holiday data from Google Calendar
2. **Calculates** which days are freeze days based on your rules  
3. **Creates** blocker events on your team calendar
4. **Manages** events automatically (add/remove as rules change)

## Commands

- `tgifreezeday sync` - Update calendar with current freeze day rules
- `tgifreezeday wipe-blockers` - Remove all managed events in time range
- `tgifreezeday list-blockers` - Show all managed events with details

## Configuration

### Basic Example

```yaml
shared:
  lookbackDays: 20      # Check 20 days back
  lookaheadDays: 60     # Check 60 days ahead
  
readFrom:
  googleCalendar:
    countryCode: "jpn"   # Japan public holidays
    todayIsFreezeDayIf:
      # - each key can be [yesterday, today, tomorrow]
      - today: [isTheFirstBusinessDayOfTheMonth]
      - today: [isTheLastBusinessDayOfTheMonth] 
      - tomorrow: [isNonBusinessDay]
      
writeTo:
  googleCalendar:
    id: "your-calendar-id@group.calendar.google.com"
    ifTodayIsFreezeDay:
      default:
        summary: "🚫 PRODUCTION FREEZE - No Deployments"
        description: |
          Production operations restricted today.<br>
          <a href="https://wiki.company.com/freeze-policy">Freeze Policy</a>
```

### Freeze Day Rules

Configure when freeze days occur using these conditions:

| Condition | Description |
|-----------|-------------|
| `isTheFirstBusinessDayOfTheMonth` | First weekday of month (excluding holidays) |
| `isTheLastBusinessDayOfTheMonth` | Last weekday of month (excluding holidays) |
| `isNonBusinessDay` | Weekend or public holiday |

### Freeze Rule Setting

The program strives to be as natural as possible. Here, in this snippet, you can kind of understand the intention:

```yaml
todayIsFreezeDayIf:
- today: [isTheFirstBusinessDayOfTheMonth]
```

> "Today is freeze-day if Today is the first business day of the month"

Here, `"today"` is a Relative Day Anchor. Here are the things you should know:
- Available Relative Day Anchor: `yesterday`, `today`, `tomorrow`
- Within an anchor, rules are AND together:
  - `today: [isA, isB]` meaning "today is freeze day if today is A and is B"
- Rules across anchors are `OR` together.
  - ```
    todayIsFreezeDayIf:
    - today: [isA, isB]
    - tomorrow: [isC]
    ```
  - Meaning, "today is freeze day if (today is A and B) OR (tomorrow is C)"

### Supported Countries

- `jpn` - Japan public holidays  
- `vnm` - Vietnam public holidays

### Rich Descriptions

HTML markup supported for calendar descriptions:

```yaml
description: |
  🚫 <strong>PRODUCTION FREEZE</strong><br><br>
  Restrictions:<br>
  <ul>
    <li>No deployments to production</li>
    <li>No infrastructure changes</li>
  </ul>
  Emergency: <a href="mailto:ops@company.com">ops@company.com</a>
```

**Supported tags**: `<br>`, `<ul><li>`, `<a href="">`, `<strong>`, `<em>`

**Note**: Avoid mixing newlines with `<br>` tags to prevent extra spacing.

## Setup, Running, Contribute

Please check [CONTRIBUTE.md](./CONTRIBUTE.md)

## License

MIT - Free to use, modify, and distribute with attribution.
