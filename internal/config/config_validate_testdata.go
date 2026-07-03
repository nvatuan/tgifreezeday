package config

import "github.com/nvat/tgifreezeday/internal/helpers"

// testAnchorToday and testCondNonBusiness are shared constants used across
// test data and test files in this package to avoid goconst threshold violations.
const (
	testAnchorToday     = "today"
	testAnchorTomorrow  = "tomorrow"
	testCondNonBusiness = "isNonBusinessDay"
)

const mockConfigYamlValid = `
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
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        summary: null
        description: null
`

var mockValidParsedConfig = &Config{
	Shared: SharedConfig{
		LookbackDays:  20,
		LookaheadDays: 60,
	},
	ReadFrom: ReadFromConfig{
		GoogleCalendar: GoogleCalendarReadConfig{
			CountryCode: "jpn",
			TodayIsFreezeDayIf: []map[string][]string{
				{
					testAnchorToday: []string{"isTheFirstBusinessDayOfTheMonth"},
				},
				{
					testAnchorToday: []string{"isTheLastBusinessDayOfTheMonth"},
				},
				{
					testAnchorTomorrow: []string{"isNonBusinessDay"},
				},
			},
		},
	},
	WriteTo: WriteToConfig{
		GoogleCalendar: GoogleCalendarWriteConfig{
			ID: "example-freeze@example.com",
			IfTodayIsFreezeDay: IfTodayIsFreezeDayConfig{
				Default: DefaultConfig{
					Summary:     helpers.StringPtr("Today is FREEZE-DAY. no PROD operation is allowed."),
					Description: helpers.StringPtr("Managed by tgifreezeday, do not modify."),
					StartTime:   helpers.StringPtr("08:00"),
					EndTime:     helpers.StringPtr("20:00"),
				},
			},
		},
	},
}

const mockConfigYamlInvalidCountryCode = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "vn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
      - today:
        - isTheLastBusinessDayOfTheMonth
      - tomorrow:
        - isNonBusinessDay
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        summary: "Today is FREEZE-DAY. no PROD operation is allowed." 
        description: "Managed by tgifreezeday, do not modify."
`

const mockConfigYamlInvalidUnsupportedDate = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "vn"
    todayIsFreezeDayIf:
      - today:
        - isTheLastBusinessDayOfTheMonth
      - nextDay:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        summary: "Today is FREEZE-DAY. no PROD operation is allowed." 
        description: "Managed by tgifreezeday, do not modify."
`

const mockConfigYamlCustomTimes = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        startTime: "09:30"
        endTime: "18:00"
`

var mockCustomTimesParsedConfig = &Config{
	Shared: SharedConfig{
		LookbackDays:  20,
		LookaheadDays: 60,
	},
	ReadFrom: ReadFromConfig{
		GoogleCalendar: GoogleCalendarReadConfig{
			CountryCode: "jpn",
			TodayIsFreezeDayIf: []map[string][]string{
				{testAnchorToday: []string{"isTheFirstBusinessDayOfTheMonth"}},
			},
		},
	},
	WriteTo: WriteToConfig{
		GoogleCalendar: GoogleCalendarWriteConfig{
			ID: "example-freeze@example.com",
			IfTodayIsFreezeDay: IfTodayIsFreezeDayConfig{
				Default: DefaultConfig{
					Summary:     helpers.StringPtr("Today is FREEZE-DAY. no PROD operation is allowed."),
					Description: helpers.StringPtr("Managed by tgifreezeday, do not modify."),
					StartTime:   helpers.StringPtr("09:30"),
					EndTime:     helpers.StringPtr("18:00"),
				},
			},
		},
	},
}

const mockConfigYamlInvalidStartTimeFormat = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        startTime: "8am"
        endTime: "20:00"
`

const mockConfigYamlInvalidEndTimeFormat = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        startTime: "08:00"
        endTime: "25:00"
`

const mockConfigYamlInvalidStartAfterEnd = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        startTime: "20:00"
        endTime: "08:00"
`

const mockConfigYamlInvalidStartEqualsEnd = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        startTime: "10:00"
        endTime: "10:00"
`

const mockConfigYamlAllDay = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "jpn"
    todayIsFreezeDayIf:
      - today:
        - isTheFirstBusinessDayOfTheMonth
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        allDay: true
`

var mockAllDayParsedConfig = &Config{
	Shared: SharedConfig{
		LookbackDays:  20,
		LookaheadDays: 60,
	},
	ReadFrom: ReadFromConfig{
		GoogleCalendar: GoogleCalendarReadConfig{
			CountryCode: "jpn",
			TodayIsFreezeDayIf: []map[string][]string{
				{testAnchorToday: []string{"isTheFirstBusinessDayOfTheMonth"}},
			},
		},
	},
	WriteTo: WriteToConfig{
		GoogleCalendar: GoogleCalendarWriteConfig{
			ID: "example-freeze@example.com",
			IfTodayIsFreezeDay: IfTodayIsFreezeDayConfig{
				Default: DefaultConfig{
					Summary:     helpers.StringPtr("Today is FREEZE-DAY. no PROD operation is allowed."),
					Description: helpers.StringPtr("Managed by tgifreezeday, do not modify."),
					AllDay:      helpers.BoolPtr(true),
				},
			},
		},
	},
}

const mockConfigYamlInvalidUnsupportedCheck = `
shared:
  lookbackDays: 20
  lookaheadDays: 60
readFrom:
  googleCalendar:
    countryCode: "vn"
    todayIsFreezeDayIf:
      - today:
        - isTheLastBusinessDayOfTheMonth
      - tomorrow:
        - isThursday
writeTo:
  googleCalendar:
    id: "example-freeze@example.com"
    ifTodayIsFreezeDay:
      default:
        summary: "Today is FREEZE-DAY. no PROD operation is allowed." 
        description: "Managed by tgifreezeday, do not modify."
`
