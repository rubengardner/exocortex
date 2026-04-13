# Jira Board Integration — Implementation Plan

Live read-only kanban view inside exocortex. Three configurable columns pulled
from the Jira REST API (v3), rendered as a new TUI state. No external binaries
required — only `net/http` with Basic Auth (email + API token).

---

## Config shape (`~/.config/exocortex/config.json`)

```json
{
  "repos": ["..."],
  "profiles": {"work": "~/.claude-work"},
  "jira": {
    "base_url":  "https://yourcompany.atlassian.net",
    "email":     "you@company.com",
    "api_token": "xxx",
    "project":   "PROJ",
    "statuses":  ["In Progress", "Ready for CR", "Code Review"]
  }
}
```

`statuses` is ordered — defines both column order and the exact status names
sent to the JQL query. Defaults to the three values above if omitted.

---

## Todos

### Phase 1 — Config

- [ ] **`internal/config/config.go`**
  - Add `JiraConfig` struct:
    ```go
    type JiraConfig struct {
        BaseURL  string   `json:"base_url"`
        Email    string   `json:"email"`
        APIToken string   `json:"api_token"`
        Project  string   `json:"project"`
        Statuses []string `json:"statuses,omitempty"`
    }
    func (j *JiraConfig) ResolvedStatuses() []string // returns Statuses or default 3
    ```
  - Add `Jira *JiraConfig `json:"jira,omitempty"`` to `Config`

---

### Phase 2 — Jira client (`internal/jira/`)

- [ ] **`internal/jira/jira.go`** — HTTP client + types
  - `Issue` struct: `Key`, `Summary`, `Status`, `Assignee` (display name), `URL`
  - `Client` struct: `baseURL`, `email`, `apiToken`
  - `New(baseURL, email, apiToken string) *Client`
  - `FetchBoard(project string, statuses []string) (map[string][]Issue, error)`
    - Builds JQL: `project=PROJ AND status in ("In Progress","Ready for CR","Code Review") ORDER BY updated DESC`
    - `GET /rest/api/3/search?jql=...&fields=summary,status,assignee&maxResults=50`
    - Basic Auth header: `Authorization: Basic base64(email:apiToken)`
    - Parses JSON response into `map[statusName][]Issue`
  - `issueURL(baseURL, key string) string` — `baseURL + "/browse/" + key`

- [ ] **`internal/jira/jira_test.go`**
  - `TestFetchBoard_ParsesResponse` — table-driven test with a fake `http.Handler`
    returning canned JSON; verify issues land in the right status bucket
  - `TestFetchBoard_HTTPError` — server returns 401/500; verify error propagates
  - `TestFetchBoard_EmptyProject` — no issues returned; verify empty map, no error
  - `TestIssueURL` — verify URL construction

---

### Phase 3 — TUI state (`internal/ui/`)

- [ ] **`internal/ui/model.go`** — new state + fields + messages
  - Add `StateJiraBoard ViewState` constant and `stateJiraBoard` alias
  - Add to `Services`:
    ```go
    LoadJiraBoard func() (map[string][]jira.Issue, error) // nil hides the binding
    ```
  - Add to `Model`:
    ```go
    jiraColumns  []string             // ordered status names (from config)
    jiraIssues   map[string][]jira.Issue
    jiraColIdx   int                  // focused column (0–2)
    jiraRowIdx   int                  // focused row within column
    jiraLoading  bool
    jiraErr      string
    jiraLastRefresh time.Time
    ```
  - Add `jiraBoardLoadedMsg` message type
  - Handle `jiraBoardLoadedMsg` in `Update` — store issues, clear loading flag
  - Add `loadJiraBoardCmd() tea.Cmd`
  - Route `stateJiraBoard` in `Update` key switch → `updateJiraBoard`
  - Route `stateJiraBoard` in `View` → `viewJiraBoard()`

- [ ] **`internal/ui/model.go`** — `updateJiraBoard`
  - `j`/`↓` — move row cursor down (clamp to column length)
  - `k`/`↑` — move row cursor up
  - `h`/`←` — move column cursor left, reset row cursor
  - `l`/`→` — move column cursor right, reset row cursor
  - `r` — reload board (`jiraLoading = true`, fire `loadJiraBoardCmd`)
  - `esc`/`q` — return to `stateList`

- [ ] **`internal/ui/model.go`** — `viewJiraBoard()`
  - Full-width 3-column layout (equal thirds)
  - Each column: header (status name, issue count), divider, then issue rows
  - Each issue row: `KEY  Summary truncated  @Assignee`
  - Selected row highlighted with `StyleSelected`
  - Focused column header highlighted with `StyleTitle`
  - Status bar shows last-refresh timestamp or loading spinner
  - If `LoadJiraBoard == nil` or `jira` config absent: show "Jira not configured" message

- [ ] **`internal/ui/keys.go`**
  - Add `Board key.Binding` — key `b`, help `"b  jira board"`
  - Add to `FullHelp` third row

- [ ] **`internal/ui/model.go`** — open board from list
  - In `updateList`: `matchKey(msg, m.keys.Board)` →
    set `stateJiraBoard`, fire `loadJiraBoardCmd` if `jiraIssues == nil`

---

### Phase 4 — Wire up (`cmd/`)

- [ ] **`cmd/ui.go`** — populate `LoadJiraBoard` in `buildServices`
  ```go
  LoadJiraBoard: func() (map[string][]jira.Issue, error) {
      cfg, err := iconfig.Load(iconfig.DefaultPath())
      if err != nil || cfg.Jira == nil {
          return nil, err
      }
      client := jira.New(cfg.Jira.BaseURL, cfg.Jira.Email, cfg.Jira.APIToken)
      return client.FetchBoard(cfg.Jira.Project, cfg.Jira.ResolvedStatuses())
  },
  ```
  - Import `github.com/ruben_gardner/exocortex/internal/jira`

---

### Phase 5 — Config for the user

- [ ] Add Jira config block to `~/.config/exocortex/config.json`

---

## Jira API reference

```
GET https://<base_url>/rest/api/3/search
Authorization: Basic <base64(email:api_token)>
Content-Type: application/json

Query params:
  jql        = project=PROJ AND status in ("s1","s2","s3") ORDER BY updated DESC
  fields     = summary,status,assignee
  maxResults = 50
```

Response shape:
```json
{
  "issues": [
    {
      "key": "PROJ-123",
      "fields": {
        "summary":  "Fix auth bug",
        "status":   { "name": "In Progress" },
        "assignee": { "displayName": "Alice Smith" }
      }
    }
  ]
}
```

API token: generate at https://id.atlassian.com/manage-profile/security/api-tokens

---

## Layout sketch

```
◈  EXOCORTEX                                   3 agent(s)
──────────────────────────────────────────────────────────
 IN PROGRESS (2)    │ READY FOR CR (1)  │ CODE REVIEW (3)
 ───────────────    │ ──────────────    │ ──────────────
▶PROJ-123           │  PROJ-456         │  PROJ-789
  Fix auth bug      │  Rate limiting    │  Review login
  @alice            │  @bob             │  @charlie
                    │                   │
  PROJ-124          │                   │  PROJ-790
  Refactor DB       │                   │  Cleanup tests
  @alice            │                   │  @dave
──────────────────────────────────────────────────────────
b back   j/k row   h/l column   r refresh
```

Column widths are equal thirds of terminal width.
Focused column header is rendered in `StyleTitle` (purple).
Selected row is rendered in `StyleSelected`.
