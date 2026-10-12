package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"time-tracking/internal/model"
	"time-tracking/internal/secrets"
	"time-tracking/internal/store"
	"time-tracking/internal/zoho"
)

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var providerErr *zoho.APIError
	if errors.As(err, &providerErr) {
		switch {
		case providerErr.Status == http.StatusRequestTimeout:
			status = http.StatusGatewayTimeout
		case providerErr.Status == 0 || providerErr.Status >= 500:
			status = http.StatusBadGateway
		case providerErr.Status == http.StatusTooManyRequests:
			status = http.StatusServiceUnavailable
		}
	}
	if errors.Is(err, store.ErrConflict) {
		status = 409
	}
	if errors.Is(err, sql.ErrNoRows) {
		status = 404
	}
	message := err.Error()
	if strings.Contains(message, "SQL") || strings.Contains(message, "constraint") {
		message = "Local storage operation failed. Your saved entries were preserved."
		status = 500
	}
	jsonResponse(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid JSON request")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}
func (a *App) Handler(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	locked := func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			r = r.WithContext(ctx)
			a.mu.Lock()
			defer a.mu.Unlock()
			fn(w, r)
		}
	}
	mux.HandleFunc("GET /api/bootstrap", locked(a.bootstrap))
	mux.HandleFunc("PUT /api/settings", locked(a.settings))
	mux.HandleFunc("GET /api/week", locked(a.getWeek))
	mux.HandleFunc("POST /api/drafts", locked(a.createDraft))
	mux.HandleFunc("PUT /api/drafts/{id}", locked(a.updateDraft))
	mux.HandleFunc("DELETE /api/drafts/{id}", locked(a.deleteDraft))
	mux.HandleFunc("POST /api/sync", locked(a.refresh))
	mux.HandleFunc("POST /api/submissions", locked(a.submitEntries))
	mux.HandleFunc("POST /api/reconcile", locked(a.reconcile))
	mux.HandleFunc("GET /api/report", locked(a.report))
	mux.HandleFunc("GET /api/backup", locked(a.backup))
	mux.HandleFunc("POST /api/connection/config", locked(a.config))
	mux.HandleFunc("POST /api/connection/auth", locked(a.auth))
	mux.HandleFunc("GET /api/connection/callback", locked(a.callback))
	mux.HandleFunc("GET /api/connection/portals", locked(a.portals))
	mux.HandleFunc("POST /api/connection/portal", locked(a.selectPortal))
	mux.HandleFunc("DELETE /api/connection", locked(a.disconnect))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]string{"error": "API endpoint not found"})
	})
	files := http.FileServer(http.FS(assets))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if _, err := fs.Stat(assets, "index.html"); err != nil {
			http.Error(w, "Build the Vue frontend first: npm run build --prefix web", 503)
			return
		}
		files.ServeHTTP(w, r)
	})
	return a.guard(mux)
}
func (a *App) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			jsonResponse(w, 403, map[string]string{"error": "This app accepts loopback hosts only"})
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Scheme != "http" || (u.Host != r.Host && u.Host != "127.0.0.1:5173" && u.Host != "localhost:5173") {
					jsonResponse(w, 403, map[string]string{"error": "Cross-origin writes are blocked"})
					return
				}
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-App-Token")), []byte(a.Token)) != 1 {
				jsonResponse(w, 403, map[string]string{"error": "Refresh the app before saving"})
				return
			}
			if r.Method != "DELETE" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				jsonResponse(w, 415, map[string]string{"error": "JSON content type required"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request) {
	configured := a.Demo
	region := a.Settings.Region
	if a.Client != nil {
		configured = a.Client.Configured()
		if region == "" {
			region = a.Client.Region()
		}
	}
	jsonResponse(w, 200, map[string]any{"settings": a.Settings, "catalog": a.Catalog, "token": a.Token, "demo": a.Demo, "configured": configured, "connected": a.connected(), "region": region, "regions": zoho.Regions, "redirectUri": a.RedirectURL})
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Timezone string `json:"timezone"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		fail(w, errors.New("choose a valid IANA timezone"))
		return
	}
	a.Settings.Timezone = input.Timezone
	if err := a.saveSettings(); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, a.Settings)
}
func (a *App) getWeek(w http.ResponseWriter, r *http.Request) {
	week, err := a.week(r.URL.Query().Get("start"))
	if err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, week)
}

type draftInput struct {
	Date      string `json:"date"`
	ProjectID string `json:"projectId"`
	TaskID    string `json:"taskId"`
	Minutes   int    `json:"minutes"`
	Billable  bool   `json:"billable"`
	Notes     string `json:"notes"`
	Revision  int    `json:"revision"`
}

func (i draftInput) draft(id string) model.Draft {
	return model.Draft{ID: id, Date: i.Date, ProjectID: i.ProjectID, TaskID: i.TaskID, Minutes: i.Minutes, Billable: i.Billable, Notes: i.Notes, Revision: i.Revision}
}
func (a *App) writeDraft(w http.ResponseWriter, r *http.Request, id string) {
	var input draftInput
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if id == "" && input.Revision != 0 {
		fail(w, errors.New("new entries must have revision zero"))
		return
	}
	d, err := a.saveDraft(input.draft(id))
	if err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, d)
}
func (a *App) createDraft(w http.ResponseWriter, r *http.Request) { a.writeDraft(w, r, "") }
func (a *App) updateDraft(w http.ResponseWriter, r *http.Request) {
	a.writeDraft(w, r, r.PathValue("id"))
}
func (a *App) deleteDraft(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
	if err != nil {
		fail(w, errors.New("revision is required"))
		return
	}
	d, err := a.Store.Draft(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	if !d.Editable() {
		fail(w, errors.New("submitted and uncertain entries are locked"))
		return
	}
	if err = a.Store.DeleteDraft(d.ID, revision); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"deleted": true})
}
func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Start string `json:"start"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if err := a.sync(r.Context(), input.Start); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"lastSync": a.Settings.LastSync})
}
func (a *App) submitEntries(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Entries []model.Selection `json:"entries"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	result, err := a.submit(r.Context(), input.Entries)
	if err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"entries": result})
}
func (a *App) reconcile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID       string `json:"id"`
		Revision int    `json:"revision"`
		Action   string `json:"action"`
		RemoteID string `json:"remoteId"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	d, err := a.Store.Draft(input.ID)
	if err != nil {
		fail(w, err)
		return
	}
	if d.Revision != input.Revision {
		fail(w, store.ErrConflict)
		return
	}
	if d.Status != "uncertain" {
		fail(w, errors.New("only uncertain entries can be reconciled"))
		return
	}
	items, err := a.candidates(r.Context(), d)
	if err != nil {
		fail(w, err)
		return
	}
	switch input.Action {
	case "check":
		jsonResponse(w, 200, map[string]any{"candidates": items, "entry": d})
		return
	case "link":
		if len(items) != 1 || items[0].ID != input.RemoteID {
			fail(w, errors.New("a single fresh matching Zoho entry is required to link"))
			return
		}
		d.Status = "submitted"
		d.RemoteID = input.RemoteID
		d.Error = ""
	case "release":
		if len(items) != 0 {
			fail(w, errors.New("matching Zoho entries exist; resolve them in Zoho before retrying"))
			return
		}
		d.Status = "failed"
		d.Error = "Checked Zoho and released for a deliberate retry."
		d.AttemptAt = ""
	default:
		fail(w, errors.New("choose check, link, or release"))
		return
	}
	if err = a.Store.SaveDraft(&d, d.Revision); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]any{"entry": d, "candidates": items})
}
func (a *App) report(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end, err := model.WeekRange(start)
	if err != nil {
		fail(w, err)
		return
	}
	entries, err := a.Store.Drafts(start, end)
	if err != nil {
		fail(w, err)
		return
	}
	text, err := model.Report(start, end, r.URL.Query().Get("projectId"), entries)
	if err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"text": text})
}
func (a *App) backup(w http.ResponseWriter, r *http.Request) {
	raw, err := a.Store.Backup()
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", "attachment; filename=tempo-backup.db")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(raw)
}
func (a *App) config(w http.ResponseWriter, r *http.Request) {
	if a.Demo {
		fail(w, errors.New("demo mode cannot connect to Zoho"))
		return
	}
	var input struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		Region       string `json:"region"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	if _, ok := zoho.Regions[input.Region]; !ok {
		fail(w, errors.New("choose your Zoho data center"))
		return
	}
	if strings.TrimSpace(input.ClientID) == "" || strings.TrimSpace(input.ClientSecret) == "" {
		fail(w, errors.New("client ID and secret are required"))
		return
	}
	if a.connected() {
		fail(w, errors.New("disconnect before replacing OAuth credentials"))
		return
	}
	c := secrets.Credentials{ClientID: strings.TrimSpace(input.ClientID), ClientSecret: strings.TrimSpace(input.ClientSecret), Region: input.Region}
	if err := a.Vault.Save(c); err != nil {
		fail(w, errors.New("could not save protected credentials"))
		return
	}
	a.Client = zoho.New(c, a.Vault.Save)
	a.Provider = a.Client
	a.oauth = map[string]oauthAttempt{}
	jsonResponse(w, 200, map[string]bool{"configured": true})
}
func (a *App) auth(w http.ResponseWriter, r *http.Request) {
	if a.Demo {
		fail(w, errors.New("demo mode cannot connect to Zoho"))
		return
	}
	state, verifier := ID(), ID()
	target, err := a.Client.AuthorizationURL(a.RedirectURL, state, verifier)
	if err != nil {
		fail(w, err)
		return
	}
	for key, attempt := range a.oauth {
		if time.Now().After(attempt.Expires) {
			delete(a.oauth, key)
		}
	}
	a.oauth[state] = oauthAttempt{Verifier: verifier, Expires: time.Now().Add(10 * time.Minute)}
	http.SetCookie(w, &http.Cookie{Name: "zoho_state", Value: state, Path: "/api/connection/callback", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	jsonResponse(w, 200, map[string]string{"url": target})
}
func (a *App) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("zoho_state")
	attempt, ok := a.oauth[state]
	delete(a.oauth, state)
	http.SetCookie(w, &http.Cookie{Name: "zoho_state", Path: "/api/connection/callback", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	if a.Demo || err != nil || !ok || time.Now().After(attempt.Expires) || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		http.Redirect(w, r, "/?connection=invalid-state", 303)
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		http.Redirect(w, r, "/?connection=denied", 303)
		return
	}
	region := r.URL.Query().Get("location")
	if region == "" {
		region = a.Client.Region()
	}
	if err = a.Client.Exchange(r.Context(), r.URL.Query().Get("code"), a.RedirectURL, attempt.Verifier, region); err != nil {
		http.Redirect(w, r, "/?connection=failed", 303)
		return
	}
	http.Redirect(w, r, "/?connection=ready", 303)
}
func (a *App) portals(w http.ResponseWriter, r *http.Request) {
	if a.Demo {
		jsonResponse(w, 200, []zoho.Portal{{ID: "demo", Name: "Studio workspace"}})
		return
	}
	p, err := a.Client.Portals(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, p)
}
func (a *App) selectPortal(w http.ResponseWriter, r *http.Request) {
	if a.Demo {
		fail(w, errors.New("demo portal is fixed"))
		return
	}
	var input struct {
		PortalID string `json:"portalId"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	p, err := a.Client.Portals(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	name := ""
	for _, portal := range p {
		if portal.ID == input.PortalID {
			name = portal.Name
		}
	}
	if name == "" {
		fail(w, errors.New("choose an accessible portal"))
		return
	}
	s, err := a.Client.CurrentUser(r.Context(), input.PortalID)
	if err != nil {
		fail(w, err)
		return
	}
	s.PortalName = name
	s.Timezone = a.Settings.Timezone
	if a.Settings.AccountKey() != s.AccountKey() {
		if err = a.Store.ReplaceLogs("0000-01-01", "9999-12-31", nil); err != nil {
			fail(w, err)
			return
		}
		a.Catalog = model.Catalog{Projects: []model.Project{}, Tasks: []model.Task{}}
		if err = a.Store.Set("catalog", a.Catalog); err != nil {
			fail(w, err)
			return
		}
	}
	a.Settings = s
	if err = a.saveSettings(); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, s)
}
func (a *App) disconnect(w http.ResponseWriter, r *http.Request) {
	if a.Demo {
		fail(w, errors.New("demo mode is already isolated"))
		return
	}
	if err := a.Vault.Clear(); err != nil {
		fail(w, errors.New("could not remove protected credentials"))
		return
	}
	a.Client = zoho.New(secrets.Credentials{}, a.Vault.Save)
	a.Provider = a.Client
	a.oauth = map[string]oauthAttempt{}
	a.Settings = model.Settings{Timezone: a.Settings.Timezone}
	a.Catalog = model.Catalog{Projects: []model.Project{}, Tasks: []model.Task{}}
	if err := a.saveSettings(); err != nil {
		fail(w, err)
		return
	}
	if err := a.Store.Set("catalog", a.Catalog); err != nil {
		fail(w, err)
		return
	}
	if err := a.Store.ReplaceLogs("0000-01-01", "9999-12-31", nil); err != nil {
		fail(w, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"disconnected": true})
}
