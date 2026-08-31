package googlecalendar

import (
	"testing"

	"google.golang.org/api/calendar/v3"
)

// jpObservanceDesc and vnObservanceDesc are the real "observance/festival" descriptions
// Google returns on the localized holiday calendars. Observances are NOT public holidays
// (they are still working days) and must be treated as non-holidays.
const (
	jpObservanceDesc = "祭日\n祭日を非表示にするには、Google カレンダーの [設定] > [日本の祝日] に移動してください"
	vnObservanceDesc = "Ngày lễ kỷ niệm\nĐể ẩn các ngày lễ kỷ niệm, hãy chuyển đến phần"
)

func TestIsPublicHoliday(t *testing.T) {
	const (
		jpCalendarID = "ja.japanese#holiday@group.v.calendar.google.com"
		vnCalendarID = "vi.vietnamese#holiday@group.v.calendar.google.com"
	)

	tests := []struct {
		name        string
		calendarID  string
		description string
		want        bool
	}{
		// Japan: localized calendar uses 祝日 for public holidays.
		{"jp public holiday", jpCalendarID, "祝日", true},
		{"jp observance is not a holiday", jpCalendarID, jpObservanceDesc, false},

		// Vietnam: localized calendar uses "Ngày lễ" for public holidays.
		// Observance ("Ngày lễ kỷ niệm") has "Ngày lễ" as a prefix, so it must not match.
		{"vn public holiday", vnCalendarID, "Ngày lễ", true},
		{"vn observance is not a holiday", vnCalendarID, vnObservanceDesc, false},

		// The old English string is never emitted by the localized calendars.
		{"legacy english string is not matched", jpCalendarID, "Public holiday", false},

		// Edge cases.
		{"empty description", jpCalendarID, "", false},
		{"unknown calendar", "en.usa#holiday@group.v.calendar.google.com", "祝日", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repository{readCalendarID: tt.calendarID}
			event := &calendar.Event{Description: tt.description}
			if got := r.isPublicHoliday(event); got != tt.want {
				t.Errorf("isPublicHoliday(%q) on %s = %v, want %v",
					tt.description, tt.calendarID, got, tt.want)
			}
		})
	}
}

// TestIsPublicHoliday_SilverWeek2026 documents the original bug: the Japanese Silver
// Week holidays (敬老の日, 国民の休日, 秋分の日) all carry DESCRIPTION:祝日 on the
// ja.japanese calendar and must be recognized as public holidays.
func TestIsPublicHoliday_SilverWeek2026(t *testing.T) {
	const jpCalendarID = "ja.japanese#holiday@group.v.calendar.google.com"
	r := &Repository{readCalendarID: jpCalendarID}

	for _, summary := range []string{"敬老の日", "国民の休日", "秋分の日"} {
		event := &calendar.Event{Summary: summary, Description: "祝日"}
		if !r.isPublicHoliday(event) {
			t.Errorf("Silver Week holiday %q with description 祝日 was not recognized as a public holiday", summary)
		}
	}
}
