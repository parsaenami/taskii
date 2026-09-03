package ui

import (
	"testing"
	"time"

	"taskii/internal/model"
)

func TestParseTaskInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		now  string
		want parsedTaskInput
	}{
		{"plain task", "Buy milk", "2026-09-03T12:00:00Z", parsedTaskInput{"Buy milk", "2026-09-03", "", model.KindTask}},
		{"legacy appointment", "Meeting 14:30", "2026-09-03T12:00:00Z", parsedTaskInput{"Meeting", "2026-09-03", "14:30", model.KindAppointment}},
		{"legacy invalid hour remains title", "Read 24:00", "2026-09-03T12:00:00Z", parsedTaskInput{"Read 24:00", "2026-09-03", "", model.KindTask}},
		{"legacy invalid minute remains title", "Read 12:60", "2026-09-03T12:00:00Z", parsedTaskInput{"Read 12:60", "2026-09-03", "", model.KindTask}},
		{"legacy short time remains title", "Read 9:30", "2026-09-03T12:00:00Z", parsedTaskInput{"Read 9:30", "2026-09-03", "", model.KindTask}},
		{"legacy signed time accepted", "Read +1:00", "2026-09-03T12:00:00Z", parsedTaskInput{"Read", "2026-09-03", "+1:00", model.KindAppointment}},
		{"date only", "Buy milk 09-05", "2026-09-03T12:00:00Z", parsedTaskInput{"Buy milk", "2026-09-05", "", model.KindTask}},
		{"date and time", "Meeting 09-05 14:30", "2026-09-03T12:00:00Z", parsedTaskInput{"Meeting", "2026-09-05", "14:30", model.KindAppointment}},
		{"same day allows earlier time", "Meeting 09-03 00:00", "2026-09-03T23:00:00Z", parsedTaskInput{"Meeting", "2026-09-03", "00:00", model.KindAppointment}},
		{"previous date rolls year", "Meeting 09-02", "2026-09-03T12:00:00Z", parsedTaskInput{"Meeting", "2027-09-02", "", model.KindTask}},
		{"new year", "Celebrate 01-01 00:00", "2026-12-31T23:59:00Z", parsedTaskInput{"Celebrate", "2027-01-01", "00:00", model.KindAppointment}},
		{"leap day from common year", "Leap day 02-29", "2026-09-03T12:00:00Z", parsedTaskInput{"Leap day", "2028-02-29", "", model.KindTask}},
		{"leap day today", "Leap day 02-29", "2028-02-29T23:59:00Z", parsedTaskInput{"Leap day", "2028-02-29", "", model.KindTask}},
		{"past leap day", "Leap day 02-29", "2028-03-01T00:00:00Z", parsedTaskInput{"Leap day", "2032-02-29", "", model.KindTask}},
		{"century leap gap", "Leap day 02-29", "2096-03-01T00:00:00Z", parsedTaskInput{"Leap day", "2104-02-29", "", model.KindTask}},
		{"four hundred year leap", "Leap day 02-29", "2399-03-01T00:00:00Z", parsedTaskInput{"Leap day", "2400-02-29", "", model.KindTask}},
		{"local day ahead of UTC", "Meeting 09-04", "2026-09-04T00:30:00+03:00", parsedTaskInput{"Meeting", "2026-09-04", "", model.KindTask}},
		{"local day behind UTC", "Meeting 09-03", "2026-09-03T23:30:00-07:00", parsedTaskInput{"Meeting", "2026-09-03", "", model.KindTask}},
		{"whitespace", "  Read  a\tbook \t09-05  14:30  ", "2026-09-03T12:00:00Z", parsedTaskInput{"Read  a\tbook", "2026-09-05", "14:30", model.KindAppointment}},
		{"date within title", "Discuss 09-05 tomorrow", "2026-09-03T12:00:00Z", parsedTaskInput{"Discuss 09-05 tomorrow", "2026-09-03", "", model.KindTask}},
		{"short date remains title", "Read 9-05", "2026-09-03T12:00:00Z", parsedTaskInput{"Read 9-05", "2026-09-03", "", model.KindTask}},
		{"non ASCII date remains title", "Read ０９-０５", "2026-09-03T12:00:00Z", parsedTaskInput{"Read ０９-０５", "2026-09-03", "", model.KindTask}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseTaskInput(tt.raw, now)
			if err != nil {
				t.Fatalf("parseTaskInput() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseTaskInput() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseTaskInputInvalid(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	for _, raw := range []string{
		"", " \t ", "14:30", "09-05", "09-05 14:30",
		"Read 00-01", "Read 13-01", "Read 01-00", "Read 01-32",
		"Read 02-30", "Read 04-31", "Read 02-30 14:30",
		"Read 09-05 24:00", "Read 09-05 12:60", "Read 09-05 xx:yy",
	} {
		t.Run(raw, func(t *testing.T) {
			if got, err := parseTaskInput(raw, now); err == nil {
				t.Errorf("parseTaskInput(%q) = %#v, expected error", raw, got)
			}
		})
	}
}
