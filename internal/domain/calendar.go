package domain

import (
	"fmt"
	"time"

	"github.com/nvat/tgifreezeday/internal/helpers"
	"github.com/nvat/tgifreezeday/internal/logging"
)

var logger = logging.GetLogger()

const notAvailable = "n/a"

// Map from date key to TGIFDay
// Allows quick lookup of TGIFDay by date key, and traverse yesterday, today, tomorrow
type DateKey string
type TGIFMapping map[DateKey]*TGIFDay

type TGIFDay struct {
	parentMapping *TGIFMapping

	Key  DateKey   // The key of the day.
	Date time.Time // The date of the day.

	// Initially nil, will be set after MonthCalendar is populated
	IsHoliday                 bool
	IsWeekend                 bool
	IsBusinessDay             bool
	IsNonBusinessDay          bool
	IsFirstBusinessDayOfMonth *bool
	IsLastBusinessDayOfMonth  *bool
}

func NewDateKey(date time.Time) DateKey {
	return DateKey(helpers.DateKey(date))
}

func NewTGIFDay(
	date time.Time,
	parentMapping *TGIFMapping,
	isHoliday bool,
) *TGIFDay {
	isWeekend := (date.Weekday() == time.Saturday || date.Weekday() == time.Sunday)

	day := &TGIFDay{
		parentMapping:    parentMapping,
		Date:             date,
		Key:              NewDateKey(date),
		IsWeekend:        isWeekend,
		IsHoliday:        isHoliday,
		IsBusinessDay:    !isWeekend && !isHoliday,
		IsNonBusinessDay: isWeekend || isHoliday,
	}
	return day
}

// FillMonthInfo fills the month info for each day in the mapping
// It sets IsFirstBusinessDayOfMonth and IsLastBusinessDayOfMonth for each day
func (m *TGIFMapping) FillMonthInfo() {
	// 2: this val is returned from date.Month()
	//  firstBusinessDay:
	//    "datekey": this datekey is the first business day of the month
	//    "n/a": cannot be computed due to lack of information because of lookback-lookahead
	//    "": not yet computed
	//  lastBusinessDay:
	//    "datekey": this datekey is the first business day of the month
	//    "n/a": cannot be computed due to lack of information because of lookback-lookahead
	//    "": not yet computed
	keyFirstBusinessDay, keyLastBusinessDay := "firstBusinessDay", "lastBusinessDay"
	monthResults := make(map[time.Month]map[string]string)

	// 1. calculate month info for each month found in the mapping. not yet setting the value in each date.
	for _, day := range *m {
		month := day.Date.Month()

		if _, ok := monthResults[month]; ok {
			continue
		}

		monthResults[month] = make(map[string]string)
		firstDayOfMonth := time.Date(day.Date.Year(), month, 1, 0, 0, 0, 0, day.Date.Location())
		firstDayOfNextMonth := time.Date(day.Date.Year(), month+1, 1, 0, 0, 0, 0, day.Date.Location())

		// compute first business day of the month
		for currDate := firstDayOfMonth; currDate.Before(firstDayOfNextMonth); currDate = currDate.AddDate(0, 0, 1) {
			dateKey := NewDateKey(currDate)

			// entry does not exist in the mapping
			v, ok := (*m)[dateKey]
			if !ok {
				monthResults[month][keyFirstBusinessDay] = notAvailable
				break
			}
			if v.IsBusinessDay {
				monthResults[month][keyFirstBusinessDay] = string(dateKey)
				break
			}
		}

		// compute last business day of the month
		for currDate := firstDayOfNextMonth; ; {
			currDate = currDate.AddDate(0, 0, -1)
			if currDate.Before(firstDayOfMonth) {
				break
			}
			// ---
			dateKey := NewDateKey(currDate)
			v, ok := (*m)[dateKey]
			if !ok {
				monthResults[month][keyLastBusinessDay] = notAvailable
				break
			}
			if v.IsBusinessDay {
				monthResults[month][keyLastBusinessDay] = string(dateKey)
				break
			}
		}
	}

	// 2. set the value in each date.
	for _, day := range *m {
		month := day.Date.Month()
		if val, ok := monthResults[month][keyFirstBusinessDay]; ok {
			if val == "" {
				panic(fmt.Sprintf("first business day of month %s is not yet computed", month))
			}
			if val == notAvailable {
				day.IsFirstBusinessDayOfMonth = helpers.BoolPtr(false) // cannot be computed, set to false.
			} else { // datekey, check if this date matches the datekey.
				day.IsFirstBusinessDayOfMonth = helpers.BoolPtr(string(day.Key) == val)
			}
		}

		if val, ok := monthResults[month][keyLastBusinessDay]; ok {
			if val == "" {
				panic(fmt.Sprintf("last business day of month %s is not yet computed", month))
			}
			if val == notAvailable {
				day.IsLastBusinessDayOfMonth = helpers.BoolPtr(false) // cannot be computed, set to false.
			} else { // datekey, check if this date matches the datekey.
				day.IsLastBusinessDayOfMonth = helpers.BoolPtr(string(day.Key) == val)
			}
		}
	}
}

func (d *TGIFDay) Offset(offsetDays int) *TGIFDay {
	offsetKey := NewDateKey(d.Date.AddDate(0, 0, offsetDays))
	v, ok := (*d.parentMapping)[offsetKey]
	if !ok {
		logger.Warnf("parentMapping does not have key %s", offsetKey)
		return nil
	}
	return v
}

func (d *TGIFDay) Yesterday() *TGIFDay {
	return d.Offset(-1)
}

func (d *TGIFDay) Today() *TGIFDay {
	return d
}

func (d *TGIFDay) Tomorrow() *TGIFDay {
	return d.Offset(1)
}

// These rules will be used in config.yaml
func (d *TGIFDay) FnIsNonBusinessDay() bool {
	if d == nil { // in case mapping does not have this date
		return false
	}
	return d.IsNonBusinessDay
}

func (d *TGIFDay) FnIsFirstBusinessDayOfMonth() bool {
	if d == nil { // in case mapping does not have this date
		return false
	}
	if d.IsFirstBusinessDayOfMonth == nil {
		panic("IsFirstBusinessDayOfMonth is nil")
	}
	return *d.IsFirstBusinessDayOfMonth
}

func (d *TGIFDay) FnIsLastBusinessDayOfMonth() bool {
	if d == nil { // in case mapping does not have this date
		return false
	}
	if d.IsLastBusinessDayOfMonth == nil {
		panic("IsLastBusinessDayOfMonth is nil")
	}
	return *d.IsLastBusinessDayOfMonth
}
