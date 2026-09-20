package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleRoutine() Routine {
	return Routine{ID: "routine-1", Title: "Walk", CreatedAt: time.Date(2026, 9, 17, 23, 30, 0, 0, time.UTC), Schedule: ScheduleWorkdays}
}

func TestRoutinePersistenceXDGAndValidation(t *testing.T) {
	dataHome, _ := isolateXDG(t)
	if got, err := LoadRoutines(); err != nil || len(got) != 0 {
		t.Fatalf("missing routines: %v %v", got, err)
	}
	r := sampleRoutine()
	r.History = map[string]RoutineStatus{"2026-09-18": RoutineCompleted}
	changeDir(t, t.TempDir())
	if err := SaveRoutines([]Routine{r}); err != nil {
		t.Fatal(err)
	}
	changeDir(t, t.TempDir())
	got, err := LoadRoutines()
	if err != nil || !reflect.DeepEqual(got, []Routine{r}) {
		t.Fatalf("round trip: %v %v", got, err)
	}
	path := filepath.Join(dataHome, appDataDir, "routines.json")
	for _, bad := range [][]byte{[]byte(`null`), []byte(`{}`), []byte(`[{"id":"x"}]`), []byte(`[{"id":"x","title":"x","created_at":"2026-09-17T00:00:00Z","schedule":"custom","custom_weekdays":[1,1]}]`)} {
		if err := os.WriteFile(path, bad, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadRoutines(); err == nil {
			t.Fatalf("accepted invalid file %s", bad)
		}
	}
	if err := SaveRoutines(nil); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "[]" {
		t.Fatalf("empty data = %s %v", b, err)
	}
	if err := SaveRoutines([]Routine{r, r}); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
}

func TestCalendarSettingsDefaultsAndExplicitSunday(t *testing.T) {
	isolateXDG(t)
	s, err := LoadSettings()
	if err != nil || s.EffectiveWeekStart() != time.Monday || !reflect.DeepEqual(s.EffectiveWorkdays(), defaultWorkdays) {
		t.Fatalf("old defaults: %+v %v", s, err)
	}
	start := time.Sunday
	s.WeekStart = &start
	s.Workdays = []time.Weekday{time.Sunday, time.Saturday}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSettings()
	if err != nil || loaded.EffectiveWeekStart() != time.Sunday || !reflect.DeepEqual(loaded.EffectiveWorkdays(), s.Workdays) {
		t.Fatalf("explicit preferences: %+v %v", loaded, err)
	}
	if b, _ := json.Marshal(loaded); !strings.Contains(string(b), `"week_start":0`) {
		t.Fatalf("Sunday disappeared in JSON: %s", b)
	}
	loaded.Workdays = []time.Weekday{}
	if err := SaveSettings(loaded); err == nil {
		t.Fatal("empty workdays accepted")
	}
	bad := time.Weekday(8)
	loaded.Workdays, loaded.WeekStart = nil, &bad
	if err := SaveSettings(loaded); err == nil {
		t.Fatal("invalid week start accepted")
	}
}

func TestScheduleAndReconcileAcrossUnscheduledDaysAndDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	r := Routine{ID: "r", Title: "Exercise", CreatedAt: time.Date(2026, 3, 6, 23, 0, 0, 0, loc), Schedule: ScheduleWorkdays,
		History: map[string]RoutineStatus{"2026-03-06": RoutineCompleted, "2026-03-09": RoutineSkipped}}
	oldDays := []time.Weekday{time.Monday, time.Friday}
	if ScheduledOnDate(r, time.Date(2026, 3, 8, 0, 0, 0, 0, loc), oldDays) {
		t.Fatal("Sunday should not be due")
	}
	changed, err := ReconcileRoutine(&r, time.Date(2026, 3, 11, 0, 1, 0, 0, loc), oldDays)
	if err != nil || !changed || r.LastEvaluatedDate != "2026-03-10" || len(r.History) != 2 || r.History["2026-03-09"] != RoutineSkipped {
		t.Fatalf("reconciliation: %+v changed=%v err=%v", r, changed, err)
	}
	changed, err = ReconcileRoutine(&r, time.Date(2026, 3, 12, 0, 0, 0, 0, loc), []time.Weekday{time.Wednesday})
	if err != nil || !changed || r.History["2026-03-11"] != RoutineMissed || r.LastEvaluatedDate != "2026-03-11" {
		t.Fatalf("new workdays: %+v changed=%v err=%v", r, changed, err)
	}
	changed, err = ReconcileRoutine(&r, time.Date(2026, 3, 12, 11, 0, 0, 0, loc), defaultWorkdays)
	if err != nil || changed || len(r.History) != 3 {
		t.Fatalf("idempotence/today pending: %+v changed=%v err=%v", r, changed, err)
	}
	r.Schedule = ScheduleCustom
	r.CustomWeekdays = []time.Weekday{time.Sunday, time.Tuesday}
	if !ScheduledOnDate(r, time.Date(2026, 3, 8, 0, 0, 0, 0, loc), nil) || ScheduledOnDate(r, time.Date(2026, 3, 9, 0, 0, 0, 0, loc), nil) {
		t.Fatal("custom weekdays ignored")
	}
}

func TestReconcileRoutineMissesAndPreservesResolvedDays(t *testing.T) {
	r := sampleRoutine()
	r.CreatedAt = time.Date(2026, 1, 30, 23, 0, 0, 0, time.UTC) // Friday
	r.History = map[string]RoutineStatus{
		"2026-01-30": RoutineCompleted,
		"2026-02-02": RoutineSkipped,
		"2026-02-03": RoutineMissed,
	}
	changed, err := ReconcileRoutine(&r, time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), nil)
	if err != nil || !changed || r.LastEvaluatedDate != "2026-02-04" || r.History["2026-02-04"] != RoutineMissed || len(r.History) != 4 {
		t.Fatalf("month boundary: %+v, changed=%t, err=%v", r, changed, err)
	}
	if r.History["2026-02-02"] != RoutineSkipped || r.History["2026-01-30"] != RoutineCompleted {
		t.Fatal("resolved history overwritten")
	}
	changed, err = ReconcileRoutine(&r, time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC), defaultWorkdays)
	if err != nil || changed || r.LastEvaluatedDate != "2026-02-04" {
		t.Fatalf("clock moving backward must not retreat cursor: %+v %t %v", r, changed, err)
	}
}
