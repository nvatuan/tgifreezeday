package domain

import "fmt"

type TodayIsFreezeDayIf []map[string][]string

// v1 config:
// // ...
// // todayIsFreezeDayIf:
// // - [yesterday, today, tomorrow]: # with this block, rules are AND together. To do OR, specify multiple items with same key.
// //   - isTheFirstBusinessDayOfTheMonth
// //   - isTheLastBusinessDayOfTheMonth
// //   - isNonBusinessDay

func getRelativeDate(relativeDate string) int {
	switch relativeDate {
	case "yesterday":
		return -1
	case "today":
		return 0
	case "tomorrow":
		return 1
	default:
		panic(fmt.Sprintf("invalid relative date: %s", relativeDate))
	}
}

func evaluateDateRule(rule string, targetDate *TGIFDay) bool {
	switch rule {
	case "isTheFirstBusinessDayOfTheMonth":
		return targetDate.FnIsFirstBusinessDayOfMonth()
	case "isTheLastBusinessDayOfTheMonth":
		return targetDate.FnIsLastBusinessDayOfMonth()
	case "isNonBusinessDay":
		return targetDate.FnIsNonBusinessDay()
	default:
		panic(fmt.Sprintf("invalid rule: %s", rule))
	}
}

// // - [yesterday, today, tomorrow]: # with this block, rules are AND together. To do OR, specify multiple items with same key.
// //   - isTheFirstBusinessDayOfTheMonth
// //   - isTheLastBusinessDayOfTheMonth
// //   - isNonBusinessDay
// within a "relativeDate" (eg. yesterday,..) rules are AND together. Each relativeDate is OR'd together.
// Consider the following to have natural understanding:
// // eg.
// // - yesterday: [isNonBusinessDay, isTheFirstBusinessDayOfTheMonth]
// // - today: [isNonBusinessDay]
// // It means: today is freeze day if:
// // // - Yesterday is non-business day and the first business day of the month (bad example)
// // // - OR, today is non-business day.
// // This is a bad example for AND, as it doesn't occur in reality so it's always false, just for the sake of example.

func (d *TGIFDay) IsTodayFreezeDay(rules TodayIsFreezeDayIf) bool {
	for _, rule := range rules {
		for relativeDate, rules := range rule {
			andResult := true
			targetDate := d.Offset(getRelativeDate(relativeDate))

			for _, rule := range rules {
				andResult = andResult && evaluateDateRule(rule, targetDate)
			}

			if andResult {
				return true // short circuit
			}
		}
	}
	return false
}
