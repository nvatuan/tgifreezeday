package config

import "github.com/nvat/tgifreezeday/internal/helpers"

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
					"today": []string{"isTheFirstBusinessDayOfTheMonth"},
				},
				{
					"today": []string{"isTheLastBusinessDayOfTheMonth"},
				},
				{
					"tomorrow": []string{"isNonBusinessDay"},
				},
			},
		},
	},
	WriteTo: WriteToConfig{
		GoogleCalendar: GoogleCalendarWriteConfig{
			ID: "example-freeze@example.com",
			IfTodayIsFreezeDay: IfTodayIsFreezeDayConfig{
				Default: DefaultConfig{
					Summary: helpers.StringPtr("Today is FREEZE-DAY. no PROD operation is allowed."),
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
`

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
`
