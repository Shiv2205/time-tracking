package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"time-tracking/internal/model"
	"time-tracking/internal/secrets"
	"time-tracking/internal/store"
	"time-tracking/internal/zoho"
)

type Provider interface {
	Catalog(context.Context, model.Settings) (model.Catalog, error)
	Logs(context.Context, model.Settings, string, string) ([]model.Log, error)
	CreateLog(context.Context, model.Settings, model.Draft) (string, error)
}
type App struct {
	mu          sync.Mutex
	Store       *store.Store
	Vault       *secrets.Vault
	Client      *zoho.Client
	Provider    Provider
	Demo        bool
	Settings    model.Settings
	Catalog     model.Catalog
	Token       string
	RedirectURL string
	oauth       map[string]oauthAttempt
}
type oauthAttempt struct {
	Verifier string
	Expires  time.Time
}

func ID() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func New(st *store.Store, vault *secrets.Vault, demo bool, redirect string) (*App, error) {
	a := &App{Store: st, Vault: vault, Demo: demo, Token: ID(), RedirectURL: redirect, oauth: map[string]oauthAttempt{}, Catalog: model.Catalog{Projects: []model.Project{}, Tasks: []model.Task{}}}
	if err := st.Get("settings", &a.Settings); err != nil {
		return nil, err
	}
	if err := st.Get("catalog", &a.Catalog); err != nil {
		return nil, err
	}
	if a.Settings.Timezone == "" {
		a.Settings.Timezone = "America/Toronto"
	}
	if demo {
		if err := a.seed(); err != nil {
			return nil, err
		}
	} else {
		creds, err := vault.Load()
		if err != nil {
			return nil, fmt.Errorf("read protected credentials: %w", err)
		}
		a.Client = zoho.New(creds, vault.Save)
		a.Provider = a.Client
	}
	return a, nil
}
func (a *App) connected() bool     { return a.Demo || (a.Client != nil && a.Client.Connected()) }
func (a *App) saveSettings() error { return a.Store.Set("settings", a.Settings) }
func (a *App) seed() error {
	a.Settings = model.Settings{Region: "demo", PortalID: "demo", PortalName: "Studio workspace", UserID: "you", UserName: "Alex Morgan", Email: "alex@example.com", Timezone: a.Settings.Timezone, LastSync: time.Now().UTC().Format(time.RFC3339)}
	a.Catalog = model.Catalog{Projects: []model.Project{{ID: "northstar", Name: "Northstar platform", Active: true}, {ID: "portal", Name: "Client portal", Active: true}, {ID: "internal", Name: "Internal operations", Active: true}}, Tasks: []model.Task{{ID: "api", ProjectID: "northstar", Name: "API integration", Assigned: true}, {ID: "design", ProjectID: "northstar", Name: "Dashboard design", Assigned: true}, {ID: "testing", ProjectID: "northstar", Name: "Quality assurance", Assigned: false}, {ID: "onboarding", ProjectID: "portal", Name: "Client onboarding", Assigned: true}, {ID: "planning", ProjectID: "internal", Name: "Sprint planning", Assigned: true}}}
	if err := a.saveSettings(); err != nil {
		return err
	}
	if err := a.Store.Set("catalog", a.Catalog); err != nil {
		return err
	}
	var seeded bool
	if err := a.Store.Get("demo_seeded", &seeded); err != nil {
		return err
	}
	if seeded {
		return nil
	}
	loc, _ := time.LoadLocation(a.Settings.Timezone)
	start := model.WeekStart(time.Now().In(loc))
	items := []model.Draft{{ProjectID: "northstar", TaskID: "api", Minutes: 180, Billable: true, Notes: "Connected project endpoints and reviewed error handling."}, {ProjectID: "northstar", TaskID: "design", Minutes: 150, Billable: true, Notes: "Explored dashboard layouts and refined the weekly view."}, {ProjectID: "portal", TaskID: "onboarding", Minutes: 120, Billable: true, Notes: "Prepared the onboarding flow and incorporated client feedback."}, {ProjectID: "internal", TaskID: "planning", Minutes: 60, Notes: "Sprint planning and team check-in."}}
	for i, d := range items {
		d.ID = ID()
		d.Date = start.AddDate(0, 0, i).Format("2006-01-02")
		d.Status = "draft"
		if err := model.Validate(&d, a.Catalog, a.Settings); err != nil {
			return err
		}
		if err := a.Store.SaveDraft(&d, 0); err != nil {
			return err
		}
	}
	date := start.Format("2006-01-02")
	logs := []model.Log{{ID: "demo-existing-1", Date: date, ProjectID: "northstar", ProjectName: "Northstar platform", TaskID: "api", TaskName: "API integration", UserID: "you", Minutes: 90, Billable: true, Notes: "Existing Zoho entry, shown for reference.", CreatedAt: time.Now().UTC().Format(time.RFC3339), Approval: "Approved"}}
	end := start.AddDate(0, 0, 6).Format("2006-01-02")
	if err := a.Store.ReplaceLogs(date, end, logs); err != nil {
		return err
	}
	return a.Store.Set("demo_seeded", true)
}
func (a *App) week(start string) (model.Week, error) {
	end, err := model.WeekRange(start)
	if err != nil {
		return model.Week{}, err
	}
	entries, err := a.Store.Drafts(start, end)
	if err != nil {
		return model.Week{}, err
	}
	logs, err := a.Store.Logs(start, end)
	if err != nil {
		return model.Week{}, err
	}
	return model.Summarize(start, end, entries, logs), nil
}
func (a *App) sync(ctx context.Context, start string) error {
	end, err := model.WeekRange(start)
	if err != nil {
		return err
	}
	if !a.connected() || a.Settings.UserID == "" {
		return errors.New("connect Zoho and select a portal first")
	}
	if !a.Demo {
		catalog, err := a.Provider.Catalog(ctx, a.Settings)
		if err != nil {
			return fmt.Errorf("could not refresh Zoho projects and tasks: %w", err)
		}
		logs, err := a.Provider.Logs(ctx, a.Settings, start, end)
		if err != nil {
			return fmt.Errorf("could not refresh Zoho time logs: %w", err)
		}
		if err = a.Store.Set("catalog", catalog); err != nil {
			return err
		}
		a.Catalog = catalog
		if err = a.Store.ReplaceLogs(start, end, logs); err != nil {
			return err
		}
	}
	a.Settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	return a.saveSettings()
}
func (a *App) saveDraft(d model.Draft) (model.Draft, error) {
	if d.ID != "" {
		old, err := a.Store.Draft(d.ID)
		if err != nil {
			return d, err
		}
		if !old.Editable() {
			return d, errors.New("submitted and uncertain entries are locked")
		}
		if d.Revision != old.Revision {
			return d, store.ErrConflict
		}
		old.Date = d.Date
		old.ProjectID = d.ProjectID
		old.TaskID = d.TaskID
		old.Minutes = d.Minutes
		old.Billable = d.Billable
		old.Notes = d.Notes
		d = old
	} else {
		d.ID = ID()
		d.Revision = 0
		d.AccountKey = ""
	}
	if err := model.Validate(&d, a.Catalog, a.Settings); err != nil {
		return d, err
	}
	d.Status = "draft"
	d.Error = ""
	d.RemoteID = ""
	d.AttemptAt = ""
	err := a.Store.SaveDraft(&d, d.Revision)
	return d, err
}
func (a *App) submit(ctx context.Context, selection []model.Selection) ([]model.Draft, error) {
	if !a.connected() {
		return nil, errors.New("connect Zoho before submitting")
	}
	if len(selection) == 0 || len(selection) > 50 {
		return nil, errors.New("select between 1 and 50 drafts")
	}
	items := []model.Draft{}
	seen := map[string]bool{}
	// Validate the entire reviewed selection before making any provider call.
	for _, s := range selection {
		if seen[s.ID] {
			return nil, errors.New("an entry was selected more than once")
		}
		seen[s.ID] = true
		d, err := a.Store.Draft(s.ID)
		if err != nil {
			return nil, err
		}
		if d.Revision != s.Revision {
			return nil, store.ErrConflict
		}
		if !d.Editable() {
			return nil, errors.New("only drafts and failed entries can be submitted")
		}
		if err = model.Validate(&d, a.Catalog, a.Settings); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	result := []model.Draft{}
	for _, d := range items {
		if err := ctx.Err(); err != nil {
			return result, nil
		}
		d.Status = "submitting"
		d.Error = ""
		d.AttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := a.Store.SaveDraft(&d, d.Revision); err != nil {
			return result, err
		}
		remoteID := "demo-" + ID()
		var err error
		if !a.Demo {
			remoteID, err = a.Provider.CreateLog(ctx, a.Settings, d)
		}
		if err == nil && remoteID == "" {
			err = &zoho.APIError{Uncertain: true}
		}
		d.Status = "submitted"
		d.RemoteID = remoteID
		if err != nil {
			d.RemoteID = ""
			d.Status = "failed"
			d.Error = err.Error()
			var api *zoho.APIError
			if errors.As(err, &api) && api.Uncertain {
				d.Status = "uncertain"
			}
		}
		if err := a.Store.SaveDraft(&d, d.Revision); err != nil {
			return result, err
		}
		result = append(result, d)
	}
	return result, nil
}
func (a *App) candidates(ctx context.Context, d model.Draft) ([]model.Log, error) {
	if d.AccountKey != a.Settings.AccountKey() {
		return nil, errors.New("reconnect the account and portal that own this entry")
	}
	if !a.connected() {
		return nil, errors.New("connect Zoho before reconciling")
	}
	var logs []model.Log
	var err error
	if a.Demo {
		logs, err = a.Store.Logs(d.Date, d.Date)
	} else {
		logs, err = a.Provider.Logs(ctx, a.Settings, d.Date, d.Date)
	}
	if err != nil {
		return nil, err
	}
	entries, err := a.Store.AllDrafts()
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, entry := range entries {
		if entry.RemoteID != "" {
			used[entry.RemoteID] = true
		}
	}
	result := []model.Log{}
	for _, candidate := range model.Candidates(d, logs, a.Settings.UserID) {
		if !used[candidate.ID] {
			result = append(result, candidate)
		}
	}
	return result, nil
}
