package model

import (
	"strings"
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	valid := map[string]int{"1:30": 90, "1.5": 90, "0:01": 1, "24": 1440, "0.25": 15, "1:20": 80}
	for input, want := range valid {
		got, err := ParseDuration(input)
		if err != nil || got != want {
			t.Errorf("%q: %d %v, want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "-1", "0", "1:60", "NaN", "999999999999999999999999:00", "24:01", "1.3333", "1e2"} {
		if _, err := ParseDuration(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
func TestWeekCalendarDST(t *testing.T) {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"2026-03-08T23:30:00", "2026-11-01T23:30:00"} {
		now, _ := time.ParseInLocation("2006-01-02T15:04:05", v, loc)
		start := WeekStart(now)
		if start.Weekday() != time.Monday || start.Hour() != 0 {
			t.Fatal(start)
		}
		end := start.AddDate(0, 0, 6)
		if end.Weekday() != time.Sunday || end.Day() != now.Day() {
			t.Fatal(end)
		}
	}
	if _, err := WeekRange("2026-10-11"); err == nil {
		t.Fatal("accepted Sunday start")
	}
	if _, err := Date("2026-02-30"); err == nil {
		t.Fatal("accepted invalid date")
	}
}
func TestSummaryAndReportDraftsOnly(t *testing.T) {
	entries := []Draft{{Date: "2026-10-05", ProjectID: "p", ProjectName: "Project", TaskName: "API", Minutes: 80, Notes: "Exact notes\nSecond line", Billable: true, Status: "draft"}, {Date: "2026-10-06", ProjectID: "p", ProjectName: "Project", TaskName: "QA", Minutes: 15, Status: "failed"}, {ProjectID: "p", Minutes: 500, Status: "submitted"}, {ProjectID: "p", Minutes: 600, Status: "uncertain"}}
	w := Summarize("2026-10-05", "2026-10-11", entries, []Log{{Minutes: 900}})
	if w.Minutes != 95 || w.BillableMinutes != 80 || w.Days[0].Minutes != 80 {
		t.Fatalf("bad totals: %+v", w)
	}
	report, err := Report(w.Start, w.End, "p", entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "Exact notes\nSecond line") || !strings.Contains(report, "Total: 1:35 hours") {
		t.Fatal(report)
	}
	if _, err := Report(w.Start, w.End, "missing", entries); err == nil {
		t.Fatal("empty report accepted")
	}
}
func TestCatalogValidationAndAccountIsolation(t *testing.T) {
	s := Settings{Region: "us", PortalID: "p", UserID: "u"}
	c := Catalog{Projects: []Project{{ID: "p1", Name: "One", Active: true}}, Tasks: []Task{{ID: "t1", Name: "Task", ProjectID: "p1"}}}
	d := Draft{Date: "2026-10-05", ProjectID: "p1", TaskID: "t1", Minutes: 45}
	if err := Validate(&d, c, s); err != nil {
		t.Fatal(err)
	}
	if d.TaskName != "Task" || d.AccountKey != s.AccountKey() {
		t.Fatal(d)
	}
	d.ProjectID = "other"
	if err := Validate(&d, c, s); err == nil {
		t.Fatal("cross-project task accepted")
	}
	d.ProjectID = "p1"
	d.AccountKey = "another-account"
	if err := Validate(&d, c, s); err == nil {
		t.Fatal("foreign account accepted")
	}
}
func TestReconciliationRequiresExactDataAndCreationWindow(t *testing.T) {
	at := time.Now().UTC()
	d := Draft{Date: "2026-10-05", ProjectID: "p", TaskID: "t", Minutes: 80, Notes: "Work", AttemptAt: at.Format(time.RFC3339Nano)}
	matching := Log{ID: "1", Date: d.Date, ProjectID: "p", TaskID: "t", UserID: "u", Minutes: 80, Notes: d.Notes, CreatedAt: at.Add(time.Second).Format(time.RFC3339Nano)}
	wrong := matching
	wrong.ID = "2"
	wrong.Minutes = 81
	old := matching
	old.ID = "3"
	old.CreatedAt = at.Add(-time.Hour).Format(time.RFC3339Nano)
	foreign := matching
	foreign.ID = "4"
	foreign.UserID = "other"
	if got := Candidates(d, []Log{matching, wrong, old, foreign}, "u"); len(got) != 1 || got[0].ID != "1" {
		t.Fatal(got)
	}
	matching.CreatedAt = ""
	if len(Candidates(d, []Log{matching}, "u")) != 0 {
		t.Fatal("missing creation time accepted")
	}
}
