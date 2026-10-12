package zoho

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"time-tracking/internal/model"
	"time-tracking/internal/secrets"
)

func testClient(server *httptest.Server) *Client {
	c := New(secrets.Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access", RefreshToken: "refresh", Region: "us", ExpiresAt: time.Now().Add(time.Hour)}, nil)
	c.ProjectsURL = server.URL
	c.AccountsURL = server.URL
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOAuthRefreshAndSafe401Retry(t *testing.T) {
	tokens, calls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/v2/token" {
			tokens++
			r.ParseForm()
			if r.Form.Get("client_secret") != "secret" || r.Form.Get("refresh_token") != "refresh" {
				t.Error("wrong token form")
			}
			w.Write([]byte(`{"access_token":"renewed","expires_in":3600}`))
			return
		}
		calls++
		if calls == 1 {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("Authorization") != "Bearer renewed" {
			t.Error("token not refreshed")
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := testClient(srv)
	if _, err := c.Portals(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tokens != 1 || calls != 2 {
		t.Fatal(tokens, calls)
	}
}
func TestAuthorizationPKCEAndExchange(t *testing.T) {
	var saved secrets.Credentials
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("code_verifier") != "verifier" || r.Form.Get("grant_type") != "authorization_code" {
			t.Error("missing verifier")
		}
		w.Write([]byte(`{"access_token":"new","refresh_token":"offline","expires_in":3600}`))
	}))
	defer srv.Close()
	c := testClient(srv)
	c.save = func(s secrets.Credentials) error { saved = s; return nil }
	target, err := c.AuthorizationURL("http://127.0.0.1:8080/api/connection/callback", "state", "verifier")
	if err != nil || !strings.Contains(target, "code_challenge_method=S256") {
		t.Fatal(target, err)
	}
	if err = c.Exchange(context.Background(), "code", "http://127.0.0.1:8080/api/connection/callback", "verifier", "ca"); err != nil {
		t.Fatal(err)
	}
	if saved.Region != "ca" || saved.RefreshToken != "offline" {
		t.Fatal("credentials not saved")
	}
}
func TestCatalogPaginationPreservesLargeIDs(t *testing.T) {
	calls := 0
	client := New(secrets.Credentials{AccessToken: "access", RefreshToken: "refresh", Region: "us", ExpiresAt: time.Now().Add(time.Hour)}, nil)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		if strings.HasSuffix(r.URL.Path, "/projects") {
			calls++
			if r.URL.Query().Get("page") == "1" {
				body = `{"projects":[{"id":170876000010385143,"name":"Project","project_type":"active"},{"id":"closed","name":"Closed","project_type":"archived"}],"page_info":{"has_next_page":true}}`
			} else {
				body = `{"projects":[],"page_info":{"has_next_page":false}}`
			}
		} else {
			if r.URL.Path != "/api/v3/portal/p/tasks" {
				t.Errorf("unexpected task endpoint: %s", r.URL.Path)
			}
			body = `{"tasks":[{"id":"170876000010385343","name":"Task","project":{"id":170876000010385143},"owners_and_work":{"owners":[{"zpuid":"u"}]}},{"id":"hidden","name":"Hidden","project":{"id":"closed"}}],"page_info":{"has_next_page":false}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	catalog, err := client.Catalog(context.Background(), model.Settings{PortalID: "p", UserID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || catalog.Projects[0].ID != "170876000010385143" || len(catalog.Tasks) != 1 || catalog.Tasks[0].ProjectID != "170876000010385143" || !catalog.Tasks[0].Assigned {
		t.Fatal(catalog, calls)
	}
}
func TestLogsFilterAndGroupedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_date") != "2026-10-05" || r.URL.Query().Get("module") != `{"type":"task"}` {
			t.Error("invalid date or module query")
		}
		var filter map[string][]string
		if json.Unmarshal([]byte(r.URL.Query().Get("filter")), &filter) != nil || filter["assignee"][0] != "u" {
			t.Error("missing current-user filter")
		}
		w.Write([]byte(`{"time_logs":[{"date":"2026-10-05","log_details":[{"id":"log","owner":{"zpuid":"u"},"project":{"id":"p","name":"Project"},"module_detail":{"id":"t","name":"Task"},"type":"task","log_hour":"01:20","billing_status":"Billable","notes":"Exact notes","created_time":"2026-10-11T16:00:00Z"},{"id":"foreign","owner":{"zpuid":"other"},"log_hour":"08:00"}]}],"page_info":{"has_next_page":false}}`))
	}))
	defer srv.Close()
	logs, err := testClient(srv).Logs(context.Background(), model.Settings{PortalID: "p", UserID: "u"}, "2026-10-05", "2026-10-11")
	if err != nil || len(logs) != 1 || logs[0].Minutes != 80 || logs[0].Date != "2026-10-05" {
		t.Fatal(logs, err)
	}
}
func TestLogsRetryWithoutAssigneeFilterOnZoho500(t *testing.T) {
	filtered, unfiltered := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "" {
			filtered++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		unfiltered++
		if r.URL.Query().Get("view_type") != "customdate" || r.URL.Query().Get("module") != `{"type":"task"}` {
			t.Error("date or module bounds missing from fallback")
		}
		w.Write([]byte(`{"time_logs":[{"date":"2026-10-05","log_details":[{"id":"own","owner":{"zpuid":"u"},"project":{"id":"p"},"module_detail":{"id":"t"},"type":"task","log_hour":"01:00"},{"id":"foreign","owner":{"zpuid":"other"},"project":{"id":"p"},"module_detail":{"id":"t"},"type":"task","log_hour":"02:00"}]}],"page_info":{"has_next_page":false}}`))
	}))
	defer srv.Close()
	c := testClient(srv)
	for i := 0; i < 2; i++ {
		logs, err := c.Logs(context.Background(), model.Settings{PortalID: "p", UserID: "u"}, "2026-10-05", "2026-10-11")
		if err != nil || len(logs) != 1 || logs[0].ID != "own" {
			t.Fatal(logs, err)
		}
	}
	if filtered != 1 || unfiltered != 2 {
		t.Fatal(filtered, unfiltered)
	}
}
func TestCreatePayloadAndUncertainFailures(t *testing.T) {
	status := 200
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v3/portal/p/projects/project/log" {
			t.Error(r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["hours"] != "1:20" || body["owner_zpuid"] != "u" || body["bill_status"] != "Billable" || object(body["module"])["type"] != "task" {
			t.Error(body)
		}
		w.WriteHeader(status)
		if status == 200 {
			w.Write([]byte(`{"id":"remote-id"}`))
		}
	}))
	defer srv.Close()
	c := testClient(srv)
	d := model.Draft{ProjectID: "project", TaskID: "task", Date: "2026-10-05", Minutes: 80, Billable: true}
	id, err := c.CreateLog(context.Background(), model.Settings{PortalID: "p", UserID: "u"}, d)
	if err != nil || id != "remote-id" {
		t.Fatal(id, err)
	}
	status = 503
	_, err = c.CreateLog(context.Background(), model.Settings{PortalID: "p", UserID: "u"}, d)
	api, ok := err.(*APIError)
	if !ok || !api.Uncertain || calls != 2 {
		t.Fatal(err, calls)
	}
}
func TestRateLimitDoesNotRetryPost(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
		w.Write([]byte(`{"secret":"do-not-expose"}`))
	}))
	defer srv.Close()
	_, err := testClient(srv).CreateLog(context.Background(), model.Settings{PortalID: "p"}, model.Draft{ProjectID: "project", TaskID: "t", Minutes: 60})
	if calls != 1 || err == nil || strings.Contains(err.Error(), "do-not-expose") || !strings.Contains(err.Error(), "120") {
		t.Fatal(calls, err)
	}
}

func TestRateLimitCooldown(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := testClient(srv)
	c.Portals(context.Background())
	if _, err := c.Portals(context.Background()); err == nil || calls != 1 {
		t.Fatal("cooldown ignored", calls, err)
	}
}
func TestCurrentUserMapsEmailToZPUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/user/info" {
			w.Write([]byte(`{"Email":"alex@example.com"}`))
			return
		}
		if r.URL.Path != "/api/v3.1/portal/p/users/alex@example.com" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(`{"id":"zp-user","full_name":"Alex Morgan","email":"alex@example.com"}`))
	}))
	defer srv.Close()
	s, err := testClient(srv).CurrentUser(context.Background(), "p")
	if err != nil || s.UserID != "zp-user" || s.Email != "alex@example.com" {
		t.Fatal(s, err)
	}
}
