package icscal

import (
	"testing"
	"time"
)

const sampleICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//test//test//EN
BEGIN:VEVENT
UID:evt-1
DTSTAMP:20240101T000000Z
DTSTART:20240615T090000Z
DTEND:20240615T100000Z
SUMMARY:Standup
LOCATION:Zoom
END:VEVENT
BEGIN:VEVENT
UID:evt-2
DTSTAMP:20240101T000000Z
DTSTART;VALUE=DATE:20240616
DTEND;VALUE=DATE:20240617
SUMMARY:Holiday
END:VEVENT
END:VCALENDAR
`

func TestParseICS(t *testing.T) {
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	evs, err := ParseICS([]byte(sampleICS), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(evs), evs)
	}
	if evs[0].Title != "Standup" || evs[0].Location != "Zoom" {
		t.Errorf("unexpected first event: %+v", evs[0])
	}
	if !evs[1].AllDay {
		t.Errorf("expected all-day event, got %+v", evs[1])
	}
}

func TestParseICSRangeFilter(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	evs, err := ParseICS([]byte(sampleICS), from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 0 {
		t.Errorf("expected no events outside range, got %d", len(evs))
	}
}
