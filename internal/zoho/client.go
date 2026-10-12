// Package zoho owns the Zoho Projects v3 wire contract. IDs never pass through float64.
package zoho

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"time-tracking/internal/model"
	"time-tracking/internal/secrets"
)

const Scopes = "ZohoProjects.portals.READ,ZohoProjects.projects.READ,ZohoProjects.tasks.READ,ZohoProjects.users.READ,ZohoProjects.timesheets.READ,ZohoProjects.timesheets.CREATE,AaaServer.profile.READ"

type Region struct {
	Name     string `json:"name"`
	Accounts string `json:"-"`
	Projects string `json:"-"`
}

var Regions = map[string]Region{
	"us": {"United States", "https://accounts.zoho.com", "https://projects.zoho.com"},
	"ca": {"Canada", "https://accounts.zohocloud.ca", "https://projects.zohocloud.ca"},
	"eu": {"European Union", "https://accounts.zoho.eu", "https://projects.zoho.eu"},
	"in": {"India", "https://accounts.zoho.in", "https://projects.zoho.in"},
	"au": {"Australia", "https://accounts.zoho.com.au", "https://projects.zoho.com.au"},
	"jp": {"Japan", "https://accounts.zoho.jp", "https://projects.zoho.jp"},
	"cn": {"China", "https://accounts.zoho.com.cn", "https://projects.zoho.com.cn"},
	"sa": {"Saudi Arabia", "https://accounts.zoho.sa", "https://projects.zoho.sa"},
	"uk": {"United Kingdom", "https://accounts.zoho.uk", "https://projects.zoho.uk"},
	"ae": {"United Arab Emirates", "https://accounts.zoho.ae", "https://projects.zoho.ae"},
	"sg": {"Singapore", "https://accounts.zoho.sg", "https://projects.zoho.sg"},
}

type Portal struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type APIError struct {
	Status     int
	Uncertain  bool
	RetryAfter string
}

func (e *APIError) Error() string {
	if e.Status == 429 {
		return "Zoho rate limit reached. Wait before refreshing or retrying (Retry-After: " + e.RetryAfter + ")."
	}
	if e.Status == 401 {
		return "Zoho authorization expired or was revoked. Reconnect your account."
	}
	if e.Status == 403 {
		return "Zoho denied access. Check your project and time-log permissions."
	}
	if e.Status == 400 || e.Status == 422 {
		return "Zoho rejected this request. Check the task, billing status, and any required time-log fields in Zoho."
	}
	if e.Uncertain {
		return "Zoho may have received this entry. Reconcile it before retrying."
	}
	if e.Status == 0 {
		return "Could not reach Zoho. Check your connection and try again."
	}
	return fmt.Sprintf("Zoho request failed (HTTP %d).", e.Status)
}

type Client struct {
	HTTP                 *http.Client
	mu                   sync.Mutex
	creds                secrets.Credentials
	save                 func(secrets.Credentials) error
	cooldown             map[string]time.Time
	logFilterUnsupported bool
	// Test servers can override hosts. Production always uses the regional allowlist.
	ProjectsURL string
	AccountsURL string
}

func New(c secrets.Credentials, save func(secrets.Credentials) error) *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, creds: c, save: save, cooldown: map[string]time.Time{}}
}
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.creds.RefreshToken != ""
}
func (c *Client) Configured() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.creds.ClientID != "" }
func (c *Client) Region() string   { c.mu.Lock(); defer c.mu.Unlock(); return c.creds.Region }
func (c *Client) hosts() (string, string) {
	r := Regions[c.creds.Region]
	a, p := r.Accounts, r.Projects
	if c.AccountsURL != "" {
		a = c.AccountsURL
	}
	if c.ProjectsURL != "" {
		p = c.ProjectsURL
	}
	return a, p
}
func (c *Client) AuthorizationURL(redirect, state, verifier string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, _ := c.hosts()
	if a == "" || c.creds.ClientID == "" {
		return "", errors.New("configure your Zoho client first")
	}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {c.creds.ClientID}, "response_type": {"code"}, "redirect_uri": {redirect}, "scope": {Scopes}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {state}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}}
	return a + "/oauth/v2/auth?" + q.Encode(), nil
}
func (c *Client) Exchange(ctx context.Context, code, redirect, verifier, region string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := Regions[region]; !ok {
		return errors.New("unsupported Zoho data center")
	}
	c.creds.Region = region
	return c.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
}
func (c *Client) token(ctx context.Context, q url.Values) error {
	a, _ := c.hosts()
	q.Set("client_id", c.creds.ClientID)
	q.Set("client_secret", c.creds.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a+"/oauth/v2/token", strings.NewReader(q.Encode()))
	if err != nil {
		return errors.New("could not prepare OAuth request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("could not reach Zoho authorization server")
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil || resp.StatusCode != 200 || result.AccessToken == "" {
		return errors.New("Zoho authorization failed. Verify client credentials, region, grant, and redirect URI.")
	}
	next := c.creds
	next.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		next.RefreshToken = result.RefreshToken
	}
	if next.RefreshToken == "" {
		return errors.New("Zoho did not return offline access. Reconnect with consent.")
	}
	if result.ExpiresIn <= 0 {
		result.ExpiresIn = 3600
	}
	next.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	if c.save != nil {
		if err = c.save(next); err != nil {
			return errors.New("could not persist protected Zoho credentials")
		}
	}
	c.creds = next
	return nil
}
func (c *Client) access(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.creds.RefreshToken == "" {
		return "", errors.New("connect your Zoho account first")
	}
	if force || time.Now().Add(time.Minute).After(c.creds.ExpiresAt) {
		if err := c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {c.creds.RefreshToken}}); err != nil {
			return "", err
		}
	}
	return c.creds.AccessToken, nil
}
func (c *Client) request(ctx context.Context, method, path string, q url.Values, body any, accounts bool) (any, error) {
	c.mu.Lock()
	until := c.cooldown[method+path]
	c.mu.Unlock()
	if remaining := time.Until(until); remaining > 0 {
		return nil, &APIError{Status: 429, RetryAfter: strconv.Itoa(int(remaining.Seconds()) + 1)}
	}
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.access(ctx, attempt == 1)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		a, p := c.hosts()
		c.mu.Unlock()
		base := p
		if accounts {
			base = a
		}
		u := base + path
		if len(q) > 0 {
			u += "?" + q.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(raw))
		if err != nil {
			return nil, errors.New("could not prepare Zoho request")
		}
		prefix := "Bearer "
		if accounts {
			prefix = "Zoho-oauthtoken "
		}
		req.Header.Set("Authorization", prefix+token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, &APIError{Uncertain: method == http.MethodPost}
		}
		if resp.StatusCode == 401 && attempt == 0 {
			resp.Body.Close()
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if resp.StatusCode == 429 {
				until := time.Now().Add(10 * time.Minute)
				if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
					until = time.Now().Add(time.Duration(seconds) * time.Second)
				} else if parsed, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil && parsed.After(time.Now()) {
					until = parsed
				}
				c.mu.Lock()
				c.cooldown[method+path] = until
				c.mu.Unlock()
			}
			resp.Body.Close()
			return nil, &APIError{Status: resp.StatusCode, Uncertain: method == http.MethodPost && (resp.StatusCode >= 500 || resp.StatusCode == 408), RetryAfter: resp.Header.Get("Retry-After")}
		}
		decoder := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
		decoder.UseNumber()
		var result any
		err = decoder.Decode(&result)
		resp.Body.Close()
		if err != nil {
			return nil, &APIError{Uncertain: method == http.MethodPost, Status: resp.StatusCode}
		}
		return result, nil
	}
	return nil, &APIError{Status: 401}
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func text(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	}
	return ""
}
func truth(v any) bool  { return v == true || v == "true" }
func array(v any) []any { a, _ := v.([]any); return a }
func (c *Client) list(ctx context.Context, path, key string, q url.Values) ([]any, error) {
	if q == nil {
		q = url.Values{}
	}
	result := []any{}
	for page := 1; page <= 1000; page++ {
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", "200")
		data, err := c.request(ctx, http.MethodGet, path, q, nil, false)
		if err != nil {
			return nil, err
		}
		m := object(data)
		items := array(data)
		if items == nil {
			items = array(m[key])
		}
		if items == nil {
			return nil, errors.New("unexpected Zoho list response; refresh did not replace cached data")
		}
		result = append(result, items...)
		info := object(m["page_info"])
		if info != nil {
			if !truth(info["has_next_page"]) {
				return result, nil
			}
		} else if len(items) < 200 {
			return result, nil
		}
	}
	return nil, errors.New("Zoho pagination exceeded the safety limit")
}
func (c *Client) Portals(ctx context.Context) ([]Portal, error) {
	items, err := c.list(ctx, "/api/v3/portals", "portals", nil)
	if err != nil {
		return nil, err
	}
	result := []Portal{}
	for _, v := range items {
		m := object(v)
		id := text(m["id"])
		name := text(m["portal_name"])
		if name == "" {
			name = text(m["name"])
		}
		if id == "" {
			return nil, errors.New("Zoho portal is missing its ID")
		}
		result = append(result, Portal{ID: id, Name: name})
	}
	return result, nil
}
func (c *Client) CurrentUser(ctx context.Context, portal string) (model.Settings, error) {
	profile, err := c.request(ctx, http.MethodGet, "/oauth/user/info", nil, nil, true)
	if err != nil {
		return model.Settings{}, err
	}
	email := text(object(profile)["Email"])
	if email == "" {
		return model.Settings{}, errors.New("Zoho did not return the account email; grant profile read access")
	}
	data, err := c.request(ctx, http.MethodGet, "/api/v3.1/portal/"+url.PathEscape(portal)+"/users/"+url.PathEscape(email), nil, nil, false)
	if err != nil {
		return model.Settings{}, err
	}
	m := object(data)
	if object(m["user"]) != nil {
		m = object(m["user"])
	}
	id := text(m["id"])
	if id == "" {
		return model.Settings{}, errors.New("could not map your Zoho account to the portal user")
	}
	return model.Settings{Region: c.Region(), PortalID: portal, UserID: id, UserName: text(m["full_name"]), Email: email}, nil
}
func (c *Client) Catalog(ctx context.Context, s model.Settings) (model.Catalog, error) {
	prefix := "/api/v3/portal/" + url.PathEscape(s.PortalID)
	items, err := c.list(ctx, prefix+"/projects", "projects", nil)
	if err != nil {
		return model.Catalog{}, fmt.Errorf("Zoho project list: %w", err)
	}
	result := model.Catalog{Projects: []model.Project{}, Tasks: []model.Task{}}
	activeProjects := map[string]bool{}
	for _, v := range items {
		m := object(v)
		id := text(m["id"])
		if id == "" {
			return result, errors.New("Zoho project is missing its ID")
		}
		active := !truth(m["is_completed"]) && !truth(object(m["status"])["is_closed_type"]) && text(m["project_type"]) != "archived"
		result.Projects = append(result.Projects, model.Project{ID: id, Name: text(m["name"]), Active: active})
		activeProjects[id] = active
	}
	tasks, err := c.list(ctx, prefix+"/tasks", "tasks", nil)
	if err != nil {
		return result, fmt.Errorf("Zoho task list: %w", err)
	}
	for _, tv := range tasks {
		t := object(tv)
		id := text(t["id"])
		projectID := text(object(t["project"])["id"])
		if id == "" || projectID == "" {
			return result, errors.New("Zoho task is missing its ID or project")
		}
		if !activeProjects[projectID] {
			continue
		}
		assigned := text(object(t["assignee"])["zpuid"]) == s.UserID
		for _, ov := range array(object(t["owners_and_work"])["owners"]) {
			if text(object(ov)["zpuid"]) == s.UserID {
				assigned = true
			}
		}
		result.Tasks = append(result.Tasks, model.Task{ID: id, ProjectID: projectID, Name: text(t["name"]), Assigned: assigned, Closed: truth(t["is_completed"]) || truth(object(t["status"])["is_closed_type"])})
	}
	return result, nil
}
func (c *Client) Logs(ctx context.Context, s model.Settings, start, end string) ([]model.Log, error) {
	filter, _ := json.Marshal(map[string]any{"assignee": []string{s.UserID}})
	module, _ := json.Marshal(map[string]string{"type": "task"})
	q := url.Values{"view_type": {"customdate"}, "start_date": {start}, "end_date": {end}, "filter": {string(filter)}, "module": {string(module)}}
	c.mu.Lock()
	withoutFilter := c.logFilterUnsupported
	c.mu.Unlock()
	if withoutFilter {
		q.Del("filter")
	}
	groups, err := c.list(ctx, "/api/v3/portal/"+url.PathEscape(s.PortalID)+"/timelogs", "time_logs", q)
	var apiErr *APIError
	if !withoutFilter && errors.As(err, &apiErr) && apiErr.Status == http.StatusInternalServerError {
		// Some Zoho portals return 500 for the documented assignee filter.
		// The date and task-module bounds remain, and owner IDs are checked below.
		q.Del("filter")
		groups, err = c.list(ctx, "/api/v3/portal/"+url.PathEscape(s.PortalID)+"/timelogs", "time_logs", q)
		if err == nil {
			c.mu.Lock()
			c.logFilterUnsupported = true
			c.mu.Unlock()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Zoho time-log list: %w", err)
	}
	result := []model.Log{}
	for _, gv := range groups {
		g := object(gv)
		details := array(g["log_details"])
		if details == nil {
			details = []any{gv}
		}
		for _, v := range details {
			m := object(v)
			user := text(object(m["owner"])["zpuid"])
			if user != s.UserID {
				continue
			}
			if typ := text(m["type"]); typ != "" && typ != "task" {
				continue
			}
			date := text(m["date"])
			if date == "" {
				date = text(g["date"])
			}
			if date < start || date > end {
				continue
			}
			minutes, err := model.ParseDuration(text(m["log_hour"]))
			if err != nil {
				return nil, errors.New("Zoho returned an unrecognized duration; cached logs were preserved")
			}
			p, t := object(m["project"]), object(m["module_detail"])
			id := text(m["id"])
			if id == "" || text(p["id"]) == "" || text(t["id"]) == "" {
				return nil, errors.New("Zoho returned an incomplete task time log")
			}
			result = append(result, model.Log{ID: id, Date: date, ProjectID: text(p["id"]), ProjectName: text(p["name"]), TaskID: text(t["id"]), TaskName: text(t["name"]), UserID: user, Minutes: minutes, Billable: text(m["billing_status"]) == "Billable", Notes: text(m["notes"]), CreatedAt: text(m["created_time"]), Approval: text(object(m["approval"])["status"])})
		}
	}
	return result, nil
}
func (c *Client) CreateLog(ctx context.Context, s model.Settings, d model.Draft) (string, error) {
	billing := "Non Billable"
	if d.Billable {
		billing = "Billable"
	}
	payload := map[string]any{"date": d.Date, "bill_status": billing, "hours": model.Duration(d.Minutes), "notes": d.Notes, "owner_zpuid": s.UserID, "module": map[string]string{"id": d.TaskID, "type": "task"}, "for_timer": false}
	data, err := c.request(ctx, http.MethodPost, "/api/v3/portal/"+url.PathEscape(s.PortalID)+"/projects/"+url.PathEscape(d.ProjectID)+"/log", nil, payload, false)
	if err != nil {
		return "", err
	}
	m := object(data)
	if object(m["time_log"]) != nil {
		m = object(m["time_log"])
	}
	id := text(m["id"])
	if id == "" {
		return "", &APIError{Status: 200, Uncertain: true}
	}
	return id, nil
}
