package jira_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/jira"
)

// --- helpers -----------------------------------------------------------------

func makeServer(statusCode int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func cannedResponse() map[string]interface{} {
	return map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"key": "PROJ-1",
				"fields": map[string]interface{}{
					"summary":  "Fix auth bug",
					"status":   map[string]interface{}{"name": "In Progress"},
					"assignee": map[string]interface{}{"displayName": "Alice Smith"},
				},
			},
			{
				"key": "PROJ-2",
				"fields": map[string]interface{}{
					"summary":  "Rate limiting",
					"status":   map[string]interface{}{"name": "Code Review"},
					"assignee": nil,
				},
			},
		},
	}
}

// --- tests -------------------------------------------------------------------

func TestFetchBoard_ParsesResponse(t *testing.T) {
	srv := makeServer(http.StatusOK, cannedResponse())
	defer srv.Close()

	c := jira.New(srv.URL, "user@example.com", "token123")
	statuses := []string{"In Progress", "Ready for CR", "Code Review"}
	board, err := c.FetchBoard(0, "PROJ", statuses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inProgress := board["In Progress"]
	if len(inProgress) != 1 {
		t.Fatalf("expected 1 issue in 'In Progress', got %d", len(inProgress))
	}
	if inProgress[0].Key != "PROJ-1" {
		t.Errorf("expected key PROJ-1, got %s", inProgress[0].Key)
	}
	if inProgress[0].Summary != "Fix auth bug" {
		t.Errorf("expected summary 'Fix auth bug', got %q", inProgress[0].Summary)
	}
	if inProgress[0].Assignee != "Alice Smith" {
		t.Errorf("expected assignee 'Alice Smith', got %q", inProgress[0].Assignee)
	}

	codeReview := board["Code Review"]
	if len(codeReview) != 1 {
		t.Fatalf("expected 1 issue in 'Code Review', got %d", len(codeReview))
	}
	if codeReview[0].Assignee != "" {
		t.Errorf("expected empty assignee for null assignee field, got %q", codeReview[0].Assignee)
	}

	readyCR := board["Ready for CR"]
	if len(readyCR) != 0 {
		t.Fatalf("expected 0 issues in 'Ready for CR', got %d", len(readyCR))
	}
}

func TestFetchBoard_HTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"server error", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := makeServer(tc.statusCode, nil)
			defer srv.Close()

			c := jira.New(srv.URL, "user@example.com", "bad-token")
			_, err := c.FetchBoard(0, "PROJ", []string{"In Progress"})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestFetchBoard_EmptyProject(t *testing.T) {
	srv := makeServer(http.StatusOK, map[string]interface{}{"issues": []interface{}{}})
	defer srv.Close()

	c := jira.New(srv.URL, "user@example.com", "token")
	board, err := c.FetchBoard(0, "PROJ", []string{"In Progress"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board["In Progress"]) != 0 {
		t.Fatalf("expected empty slice, got %d issues", len(board["In Progress"]))
	}
}

func TestIssueURL(t *testing.T) {
	srv := makeServer(http.StatusOK, cannedResponse())
	defer srv.Close()

	c := jira.New(srv.URL, "user@example.com", "token")
	board, err := c.FetchBoard(0, "PROJ", []string{"In Progress", "Ready for CR", "Code Review"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	issue := board["In Progress"][0]
	want := srv.URL + "/browse/PROJ-1"
	if issue.URL != want {
		t.Errorf("expected URL %s, got %s", want, issue.URL)
	}
}
