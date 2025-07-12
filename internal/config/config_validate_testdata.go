package config

import "github.com/nvat/tgifreezeday/internal/helpers"

const mock_configYamlValid = `
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

const mock_configYamlInvalid_countryCode = `
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

const mock_configYamlInvalid_unsupportedDate = `
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

const mock_configYamlInvalid_unsupportedCheck = `
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
