package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	igithub "github.com/ruben_gardner/exocortex/internal/github"
)

// --- helpers -----------------------------------------------------------------

type testMux struct {
	paths map[string]http.HandlerFunc
}

func newTestMux() *testMux { return &testMux{paths: make(map[string]http.HandlerFunc)} }

func (m *testMux) handle(path string, fn http.HandlerFunc) { m.paths[path] = fn }

func (m *testMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fn, ok := m.paths[r.URL.Path]; ok {
		fn(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func jsonResp(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// makeSearchItem builds a minimal search API item payload.
func makeSearchItem(number int, title, login, repo, head, base string, draft bool, mergedAt interface{}) map[string]interface{} {
	return map[string]interface{}{
		"number":     number,
		"title":      title,
		"state":      "open",
		"draft":      draft,
		"html_url":   "https://github.com/" + repo + "/pull/42",
		"merged_at":  mergedAt,
		"updated_at": "2024-01-15T12:00:00Z",
		"user":       map[string]interface{}{"login": login},
		"head": map[string]interface{}{
			"ref":  head,
			"repo": map[string]interface{}{"full_name": repo},
		},
		"base": map[string]interface{}{"ref": base},
		"pull_request": map[string]interface{}{
			"url": "https://api.github.com/repos/" + repo + "/pulls/42",
		},
	}
}

// --- TestListPRs -------------------------------------------------------------

func TestListPRs_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, map[string]interface{}{
			"items": []interface{}{
				makeSearchItem(42, "Fix auth bug", "alice", "org/repo", "fix-auth", "main", false, nil),
			},
		})
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "mytoken", "")
	prs, err := c.ListPRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	pr := prs[0]
	if pr.Number != 42 {
		t.Errorf("Number: got %d, want 42", pr.Number)
	}
	if pr.Title != "Fix auth bug" {
		t.Errorf("Title: got %q, want 'Fix auth bug'", pr.Title)
	}
	if pr.Author != "alice" {
		t.Errorf("Author: got %q, want 'alice'", pr.Author)
	}
	if pr.Repo != "org/repo" {
		t.Errorf("Repo: got %q, want 'org/repo'", pr.Repo)
	}
	if pr.Branch != "fix-auth" {
		t.Errorf("Branch: got %q, want 'fix-auth'", pr.Branch)
	}
	if pr.Base != "main" {
		t.Errorf("Base: got %q, want 'main'", pr.Base)
	}
	if pr.State != "open" {
		t.Errorf("State: got %q, want 'open'", pr.State)
	}
	if pr.IsDraft {
		t.Error("IsDraft: got true, want false")
	}
	wantTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	if !pr.UpdatedAt.Equal(wantTime) {
		t.Errorf("UpdatedAt: got %v, want %v", pr.UpdatedAt, wantTime)
	}
}

func TestListPRs_SendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		jsonResp(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}})
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "secret-token", "")
	_, _ = c.ListPRs()

	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer secret-token")
	}
}

func TestListPRs_OrgAddsToQuery(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		jsonResp(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}})
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "myorg")
	_, _ = c.ListPRs()

	decoded, err := url.QueryUnescape(gotRawQuery)
	if err != nil {
		t.Fatalf("could not unescape query %q: %v", gotRawQuery, err)
	}
	if !strings.Contains(decoded, "org:myorg") {
		t.Errorf("query %q does not contain 'org:myorg'", decoded)
	}
}

func TestListPRs_DetectsMergedState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		item := makeSearchItem(1, "Merged PR", "bob", "org/repo", "feat", "main", false, "2024-01-10T10:00:00Z")
		jsonResp(w, http.StatusOK, map[string]interface{}{"items": []interface{}{item}})
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "")
	prs, err := c.ListPRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].State != "merged" {
		t.Errorf("State: got %q, want 'merged'", prs[0].State)
	}
}

func TestListPRs_DetectsDraftState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		item := makeSearchItem(2, "Draft PR", "carol", "org/repo", "wip", "main", true, nil)
		jsonResp(w, http.StatusOK, map[string]interface{}{"items": []interface{}{item}})
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "")
	prs, err := c.ListPRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].State != "draft" {
		t.Errorf("State: got %q, want 'draft'", prs[0].State)
	}
	if !prs[0].IsDraft {
		t.Error("IsDraft: got false, want true")
	}
}

func TestListPRs_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "bad-token", "")
	_, err := c.ListPRs()
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

// --- TestFetchPRDetail -------------------------------------------------------

func prDetailPayload() map[string]interface{} {
	return map[string]interface{}{
		"number":        42,
		"title":         "Fix auth bug",
		"state":         "open",
		"draft":         false,
		"body":          "## Summary\nFixes the login flow.",
		"html_url":      "https://github.com/org/repo/pull/42",
		"additions":     15,
		"deletions":     3,
		"changed_files": 2,
		"updated_at":    "2024-01-15T12:00:00Z",
		"merged_at":     nil,
		"user":          map[string]interface{}{"login": "alice"},
		"head": map[string]interface{}{
			"ref":  "fix-auth",
			"repo": map[string]interface{}{"full_name": "org/repo"},
		},
		"base": map[string]interface{}{"ref": "main"},
	}
}

func prFilesPayload() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"filename":  "internal/auth/auth.go",
			"status":    "modified",
			"additions": 10,
			"deletions": 2,
			"patch":     "@@ -1,5 +1,13 @@\n func Login() {}",
		},
		map[string]interface{}{
			"filename":  "internal/auth/auth_test.go",
			"status":    "added",
			"additions": 5,
			"deletions": 1,
			"patch":     "@@ -0,0 +1,5 @@\n func TestLogin() {}",
		},
	}
}

func TestFetchPRDetail_ParsesFields(t *testing.T) {
	mx := newTestMux()
	mx.handle("/repos/org/repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, prDetailPayload())
	})
	mx.handle("/repos/org/repo/pulls/42/files", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, prFilesPayload())
	})
	srv := httptest.NewServer(mx)
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "")
	detail, err := c.FetchPRDetail("org/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Number != 42 {
		t.Errorf("Number: got %d, want 42", detail.Number)
	}
	if detail.Title != "Fix auth bug" {
		t.Errorf("Title: got %q, want 'Fix auth bug'", detail.Title)
	}
	if detail.Body != "## Summary\nFixes the login flow." {
		t.Errorf("Body: got %q", detail.Body)
	}
	if detail.Additions != 15 {
		t.Errorf("Additions: got %d, want 15", detail.Additions)
	}
	if detail.Deletions != 3 {
		t.Errorf("Deletions: got %d, want 3", detail.Deletions)
	}
	if detail.ChangedFiles != 2 {
		t.Errorf("ChangedFiles: got %d, want 2", detail.ChangedFiles)
	}
	if detail.Branch != "fix-auth" {
		t.Errorf("Branch: got %q, want 'fix-auth'", detail.Branch)
	}
	if detail.Base != "main" {
		t.Errorf("Base: got %q, want 'main'", detail.Base)
	}
}

func TestFetchPRDetail_ParsesFiles(t *testing.T) {
	mx := newTestMux()
	mx.handle("/repos/org/repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, prDetailPayload())
	})
	mx.handle("/repos/org/repo/pulls/42/files", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, http.StatusOK, prFilesPayload())
	})
	srv := httptest.NewServer(mx)
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "")
	detail, err := c.FetchPRDetail("org/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detail.Files) != 2 {
		t.Fatalf("Files: got %d, want 2", len(detail.Files))
	}
	f0 := detail.Files[0]
	if f0.Path != "internal/auth/auth.go" {
		t.Errorf("Files[0].Path: got %q, want 'internal/auth/auth.go'", f0.Path)
	}
	if f0.Status != "modified" {
		t.Errorf("Files[0].Status: got %q, want 'modified'", f0.Status)
	}
	if f0.Additions != 10 {
		t.Errorf("Files[0].Additions: got %d, want 10", f0.Additions)
	}
	if f0.Deletions != 2 {
		t.Errorf("Files[0].Deletions: got %d, want 2", f0.Deletions)
	}
	if f0.Patch != "@@ -1,5 +1,13 @@\n func Login() {}" {
		t.Errorf("Files[0].Patch: got %q", f0.Patch)
	}

	f1 := detail.Files[1]
	if f1.Status != "added" {
		t.Errorf("Files[1].Status: got %q, want 'added'", f1.Status)
	}
}

func TestFetchPRDetail_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := igithub.New(srv.URL, "token", "")
	_, err := c.FetchPRDetail("org/repo", 99)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}
