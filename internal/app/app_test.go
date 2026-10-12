package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"time-tracking/internal/model"
	"time-tracking/internal/secrets"
	"time-tracking/internal/store"
	"time-tracking/internal/zoho"
)

func demoApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	a, err := New(st, secrets.New(dir), true, "http://127.0.0.1:8080/api/connection/callback")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

type fakeProvider struct {
	calls      int
	logs       []model.Log
	catalogErr error
	logsErr    error
}

func (f *fakeProvider) Catalog(context.Context, model.Settings) (model.Catalog, error) {
	return model.Catalog{}, f.catalogErr
}
func (f *fakeProvider) Logs(context.Context, model.Settings, string, string) ([]model.Log, error) {
	return f.logs, f.logsErr
}
func (f *fakeProvider) CreateLog(context.Context, model.Settings, model.Draft) (string, error) {
	f.calls++
	switch f.calls {
	case 1:
		return "confirmed", nil
	case 2:
		return "", &zoho.APIError{Status: 503, Uncertain: true}
	default:
		return "", &zoho.APIError{Status: 400}
	}
}
func connectFake(a *App, f Provider) {
	a.Demo = false
	a.Client = zoho.New(secrets.Credentials{ClientID: "id", RefreshToken: "refresh"}, nil)
	a.Provider = f
}
func TestPartialSubmissionAndNoBlindRetry(t *testing.T) {
	a := demoApp(t)
	f := &fakeProvider{}
	connectFake(a, f)
	entries, _ := a.Store.AllDrafts()
	selection := []model.Selection{}
	for _, e := range entries {
		selection = append(selection, model.Selection{ID: e.ID, Revision: e.Revision})
	}
	result, err := a.submit(context.Background(), selection[:3])
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Status != "submitted" || result[1].Status != "uncertain" || result[2].Status != "failed" {
		t.Fatal(result)
	}
	for _, e := range result[:2] {
		_, err = a.submit(context.Background(), []model.Selection{{ID: e.ID, Revision: e.Revision}})
		if err == nil {
			t.Fatal("locked entry resubmitted")
		}
		if _, err = a.saveDraft(e); err == nil {
			t.Fatal("locked entry edited")
		}
	}
	if f.calls != 3 {
		t.Fatal(f.calls)
	}
}
func TestStaleBatchPreventsAnyWrites(t *testing.T) {
	a := demoApp(t)
	f := &fakeProvider{}
	connectFake(a, f)
	entries, _ := a.Store.AllDrafts()
	_, err := a.submit(context.Background(), []model.Selection{{ID: entries[0].ID, Revision: entries[0].Revision}, {ID: entries[1].ID, Revision: 99}})
	if !errors.Is(err, store.ErrConflict) || f.calls != 0 {
		t.Fatal(err, f.calls)
	}
}
func TestSaveAndReadOnlyOperationsNeverWriteProvider(t *testing.T) {
	a := demoApp(t)
	f := &fakeProvider{}
	connectFake(a, f)
	entries, _ := a.Store.AllDrafts()
	d := entries[0]
	d.Notes = "Edited"
	if _, err := a.saveDraft(d); err != nil {
		t.Fatal(err)
	}
	start := model.WeekStart(time.Now()).Format("2006-01-02")
	if _, err := a.week(start); err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 {
		t.Fatal("save called provider")
	}
}
func TestSyncReportsProviderStageAndGatewayStatus(t *testing.T) {
	a := demoApp(t)
	f := &fakeProvider{catalogErr: &zoho.APIError{Status: 500}}
	connectFake(a, f)
	handler := a.Handler(fstest.MapFS{})
	start := model.WeekStart(time.Now()).Format("2006-01-02")
	request := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"start": start})
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/sync", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-App-Token", a.Token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request(); w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "Zoho projects and tasks") {
		t.Fatal(w.Code, w.Body.String())
	}
	f.catalogErr = nil
	f.logsErr = &zoho.APIError{Status: 500}
	if w := request(); w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "Zoho time logs") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestBrowserWriteGuardAndSecretRedaction(t *testing.T) {
	a := demoApp(t)
	handler := a.Handler(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("app")}})
	cases := []struct {
		host, origin, token string
		want                int
	}{{"evil.example:8080", "", a.Token, 403}, {"127.0.0.1:8080", "https://evil.example", a.Token, 403}, {"127.0.0.1:8080", "http://127.0.0.1:8080", "", 403}, {"127.0.0.1:8080", "http://127.0.0.1:8080", a.Token, 200}}
	for _, c := range cases {
		r := httptest.NewRequest("PUT", "http://127.0.0.1:8080/api/settings", strings.NewReader(`{"timezone":"America/Toronto"}`))
		r.Host = c.host
		r.Header.Set("Origin", c.origin)
		r.Header.Set("X-App-Token", c.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Fatal(c, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/bootstrap", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "clientSecret") || strings.Contains(w.Body.String(), "refreshToken") {
		t.Fatal("credentials exposed")
	}
}
func TestOAuthInvalidStateDoesNotExchange(t *testing.T) {
	a := demoApp(t)
	a.Demo = false
	a.Client = zoho.New(secrets.Credentials{}, nil)
	a.oauth["valid"] = oauthAttempt{Verifier: "v", Expires: time.Now().Add(time.Minute)}
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/connection/callback?code=code&state=valid", nil)
	r.AddCookie(&http.Cookie{Name: "zoho_state", Value: "wrong"})
	w := httptest.NewRecorder()
	a.callback(w, r)
	if w.Header().Get("Location") != "/?connection=invalid-state" {
		t.Fatal(w.Header())
	}
}
func TestHTTPRevisionCheckedDeletion(t *testing.T) {
	a := demoApp(t)
	entries, _ := a.Store.AllDrafts()
	handler := a.Handler(fstest.MapFS{})
	r := httptest.NewRequest("DELETE", "http://127.0.0.1:8080/api/drafts/"+entries[0].ID+"?revision=99", nil)
	r.Header.Set("X-App-Token", a.Token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 409 {
		body, _ := io.ReadAll(w.Body)
		t.Fatal(w.Code, string(body))
	}
	if _, err := a.Store.Draft(entries[0].ID); err != nil {
		t.Fatal("draft lost")
	}
}

func TestReconciliationCheckLinkAndAmbiguity(t *testing.T) {
	a := demoApp(t)
	f := &fakeProvider{}
	connectFake(a, f)
	entries, _ := a.Store.AllDrafts()
	d := entries[0]
	d.Status = "uncertain"
	d.AttemptAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := a.Store.SaveDraft(&d, d.Revision); err != nil {
		t.Fatal(err)
	}
	log := model.Log{ID: "match", Date: d.Date, ProjectID: d.ProjectID, TaskID: d.TaskID, UserID: a.Settings.UserID, Minutes: d.Minutes, Billable: d.Billable, Notes: d.Notes, CreatedAt: d.AttemptAt}
	f.logs = []model.Log{log}
	request := func(action, id string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"id": d.ID, "revision": d.Revision, "action": action, "remoteId": id})
		r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/reconcile", bytes.NewReader(body))
		w := httptest.NewRecorder()
		a.reconcile(w, r)
		return w
	}
	if w := request("check", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "match") {
		t.Fatal(w.Code, w.Body.String())
	}
	second := log
	second.ID = "another-match"
	f.logs = append(f.logs, second)
	if w := request("link", "match"); w.Code != 400 {
		t.Fatal("ambiguous match linked", w.Code)
	}
	if w := request("release", ""); w.Code != 400 {
		t.Fatal("existing match released", w.Code)
	}
	f.logs = []model.Log{log}
	if w := request("link", "match"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	got, _ := a.Store.Draft(d.ID)
	if got.Status != "submitted" || got.RemoteID != "match" {
		t.Fatal(got)
	}
	if f.calls != 0 {
		t.Fatal("reconciliation wrote to provider")
	}
}
