package model

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Settings struct {
	Region     string `json:"region"`
	PortalID   string `json:"portalId"`
	PortalName string `json:"portalName"`
	UserID     string `json:"userId"`
	UserName   string `json:"userName"`
	Email      string `json:"email"`
	Timezone   string `json:"timezone"`
	LastSync   string `json:"lastSync"`
}

func (s Settings) AccountKey() string { return s.Region + "/" + s.PortalID + "/" + s.UserID }

type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
type Task struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Assigned  bool   `json:"assigned"`
	Closed    bool   `json:"closed"`
}
type Catalog struct {
	Projects []Project `json:"projects"`
	Tasks    []Task    `json:"tasks"`
}
type Draft struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	TaskID      string `json:"taskId"`
	TaskName    string `json:"taskName"`
	Minutes     int    `json:"minutes"`
	Billable    bool   `json:"billable"`
	Notes       string `json:"notes"`
	Status      string `json:"status"`
	Revision    int    `json:"revision"`
	AccountKey  string `json:"accountKey"`
	RemoteID    string `json:"remoteId"`
	Error       string `json:"error"`
	AttemptAt   string `json:"attemptAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func (d Draft) Editable() bool { return d.Status == "draft" || d.Status == "failed" }

type Log struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	TaskID      string `json:"taskId"`
	TaskName    string `json:"taskName"`
	UserID      string `json:"userId"`
	Minutes     int    `json:"minutes"`
	Billable    bool   `json:"billable"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"createdAt"`
	Approval    string `json:"approval"`
}
type Selection struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
type Total struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Minutes int    `json:"minutes"`
}
type Week struct {
	Start           string  `json:"start"`
	End             string  `json:"end"`
	Entries         []Draft `json:"entries"`
	Logs            []Log   `json:"logs"`
	Minutes         int     `json:"minutes"`
	BillableMinutes int     `json:"billableMinutes"`
	Days            []Total `json:"days"`
	Projects        []Total `json:"projects"`
}

var durationPattern = regexp.MustCompile(`^\d+(?::[0-5]\d|(?:\.\d{1,4})?)$`)

func ParseDuration(v string) (int, error) {
	v = strings.TrimSpace(v)
	if !durationPattern.MatchString(v) {
		return 0, errors.New("enter a duration such as 1:30 or 1.5")
	}
	var minutes int
	if parts := strings.Split(v, ":"); len(parts) == 2 {
		h, err := strconv.Atoi(parts[0])
		if err != nil || h > 24 {
			return 0, errors.New("duration must be between 1 minute and 24 hours")
		}
		m, _ := strconv.Atoi(parts[1])
		minutes = h*60 + m
	} else {
		h, err := strconv.ParseFloat(v, 64)
		if err != nil || h > 24 {
			return 0, errors.New("duration must be between 1 minute and 24 hours")
		}
		n := h * 60
		if math.Abs(n-math.Round(n)) > 0.000001 {
			return 0, errors.New("duration must represent whole minutes; use H:MM")
		}
		minutes = int(math.Round(n))
	}
	if minutes < 1 || minutes > 1440 {
		return 0, errors.New("duration must be between 1 minute and 24 hours")
	}
	return minutes, nil
}
func Duration(minutes int) string { return fmt.Sprintf("%d:%02d", minutes/60, minutes%60) }
func Date(v string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", v)
	if err != nil || t.Format("2006-01-02") != v {
		return time.Time{}, errors.New("use a valid YYYY-MM-DD date")
	}
	return t, nil
}
func WeekStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day()-((int(t.Weekday())+6)%7), 0, 0, 0, 0, t.Location())
}
func WeekRange(start string) (string, error) {
	t, err := Date(start)
	if err != nil {
		return "", err
	}
	if t.Weekday() != time.Monday {
		return "", errors.New("week must begin on Monday")
	}
	return t.AddDate(0, 0, 6).Format("2006-01-02"), nil
}
func Validate(d *Draft, c Catalog, s Settings) error {
	if _, err := Date(d.Date); err != nil {
		return err
	}
	if d.Minutes < 1 || d.Minutes > 1440 {
		return errors.New("duration must be between 1 minute and 24 hours")
	}
	if !utf8.ValidString(d.Notes) || utf8.RuneCountInString(d.Notes) > 10000 {
		return errors.New("notes must contain at most 10,000 characters")
	}
	if s.UserID == "" || s.PortalID == "" {
		return errors.New("connect a Zoho account and select a portal first")
	}
	if d.AccountKey != "" && d.AccountKey != s.AccountKey() {
		return errors.New("this draft belongs to a different Zoho account or portal")
	}
	pFound, tFound := false, false
	for _, p := range c.Projects {
		if p.ID == d.ProjectID && p.Active {
			pFound = true
			d.ProjectName = p.Name
		}
	}
	for _, t := range c.Tasks {
		if t.ID == d.TaskID && t.ProjectID == d.ProjectID {
			tFound = true
			d.TaskName = t.Name
		}
	}
	if !pFound || !tFound {
		return errors.New("choose an available project and a task belonging to that project")
	}
	d.AccountKey = s.AccountKey()
	return nil
}
func Summarize(start, end string, entries []Draft, logs []Log) Week {
	w := Week{Start: start, End: end, Entries: entries, Logs: logs, Days: []Total{}, Projects: []Total{}}
	day := map[string]int{}
	project := map[string]Total{}
	for _, d := range entries {
		if !d.Editable() {
			continue
		}
		w.Minutes += d.Minutes
		if d.Billable {
			w.BillableMinutes += d.Minutes
		}
		day[d.Date] += d.Minutes
		p := project[d.ProjectID]
		p.Key = d.ProjectID
		p.Name = d.ProjectName
		p.Minutes += d.Minutes
		project[d.ProjectID] = p
	}
	t, _ := Date(start)
	for i := 0; i < 7; i++ {
		date := t.AddDate(0, 0, i).Format("2006-01-02")
		w.Days = append(w.Days, Total{Key: date, Name: date, Minutes: day[date]})
	}
	for _, p := range project {
		w.Projects = append(w.Projects, p)
	}
	sort.Slice(w.Projects, func(i, j int) bool { return w.Projects[i].Name < w.Projects[j].Name })
	return w
}
func Report(start, end, projectID string, entries []Draft) (string, error) {
	items := []Draft{}
	for _, d := range entries {
		if d.ProjectID == projectID && d.Editable() {
			items = append(items, d)
		}
	}
	if len(items) == 0 {
		return "", errors.New("no drafts for this project and week")
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Date == items[j].Date {
			return items[i].TaskName < items[j].TaskName
		}
		return items[i].Date < items[j].Date
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s — weekly work report\n%s to %s\nLocal drafts · not yet submitted to Zoho\n\n", items[0].ProjectName, start, end)
	total := 0
	for _, d := range items {
		fmt.Fprintf(&b, "%s · %s · %s\n", d.Date, d.TaskName, Duration(d.Minutes))
		if d.Notes != "" {
			b.WriteString(d.Notes)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
		total += d.Minutes
	}
	fmt.Fprintf(&b, "Total: %s hours", Duration(total))
	return b.String(), nil
}

// Candidates never infer a match from a remote log lacking a trustworthy creation time.
func Candidates(d Draft, logs []Log, userID string) []Log {
	result := []Log{}
	attempt, err := time.Parse(time.RFC3339Nano, d.AttemptAt)
	if err != nil {
		return result
	}
	for _, l := range logs {
		created, err := time.Parse(time.RFC3339Nano, l.CreatedAt)
		if err != nil {
			continue
		}
		if l.UserID == userID && l.ProjectID == d.ProjectID && l.TaskID == d.TaskID && l.Date == d.Date && l.Minutes == d.Minutes && l.Billable == d.Billable && l.Notes == d.Notes && !created.Before(attempt.Add(-2*time.Minute)) && !created.After(attempt.Add(5*time.Minute)) {
			result = append(result, l)
		}
	}
	return result
}
