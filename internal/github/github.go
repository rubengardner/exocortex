package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PR represents a pull request as returned by the GitHub search API.
type PR struct {
	Number    int
	Title     string
	Author    string
	Repo      string
	Branch    string
	Base      string
	State     string // "open" | "draft" | "merged" | "closed"
	IsDraft   bool
	UpdatedAt time.Time
	URL       string
}

// PRDetail extends PR with full description and file diff information.
type PRDetail struct {
	PR
	Body         string
	Additions    int
	Deletions    int
	ChangedFiles int
	Files        []PRFile
}

// PRFile is a single changed file within a PR.
type PRFile struct {
	Path      string
	Status    string // "added" | "modified" | "removed" | "renamed" | ...
	Additions int
	Deletions int
	Patch     string
}

// Client is the GitHub API client.
type Client struct {
	baseURL string
	token   string
	org     string
	http    *http.Client
}

// New returns a new Client. baseURL is "https://api.github.com" in production;
// inject a test server URL in tests.
func New(baseURL, token, org string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		org:     org,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// ListPRs fetches open PRs involving the authenticated user via the GitHub
// search/issues endpoint.
func (c *Client) ListPRs() ([]PR, error) {
	q := "is:pr is:open involves:@me"
	if c.org != "" {
		q += " org:" + c.org
	}

	endpoint := c.baseURL + "/search/issues?q=" + url.QueryEscape(q) + "&sort=updated&per_page=50"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: list PRs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: list PRs: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Items []searchItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("github: list PRs: decode: %w", err)
	}

	prs := make([]PR, 0, len(payload.Items))
	for _, it := range payload.Items {
		prs = append(prs, it.toPR())
	}
	return prs, nil
}

// FetchPRDetail fetches full PR details including file diffs.
// repo is "owner/name", e.g. "org/repo".
func (c *Client) FetchPRDetail(repo string, number int) (*PRDetail, error) {
	prURL := fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, repo, number)
	req, err := http.NewRequest(http.MethodGet, prURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: fetch PR %d: %w", number, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: fetch PR %d: HTTP %d", number, resp.StatusCode)
	}

	var raw prDetailRaw
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: fetch PR %d: decode: %w", number, err)
	}

	// Fetch changed files.
	filesURL := fmt.Sprintf("%s/repos/%s/pulls/%d/files", c.baseURL, repo, number)
	req2, err := http.NewRequest(http.MethodGet, filesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build files request: %w", err)
	}
	req2.Header.Set("Authorization", "Bearer "+c.token)
	req2.Header.Set("Accept", "application/vnd.github+json")

	resp2, err := c.http.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("github: fetch PR %d files: %w", number, err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: fetch PR %d files: HTTP %d", number, resp2.StatusCode)
	}

	var rawFiles []prFileRaw
	if err := json.NewDecoder(resp2.Body).Decode(&rawFiles); err != nil {
		return nil, fmt.Errorf("github: fetch PR %d files: decode: %w", number, err)
	}

	detail := raw.toPRDetail()
	detail.Files = make([]PRFile, 0, len(rawFiles))
	for _, f := range rawFiles {
		detail.Files = append(detail.Files, PRFile{
			Path:      f.Filename,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
			Patch:     f.Patch,
		})
	}
	return detail, nil
}

// repoFromPRURL extracts "owner/repo" from a GitHub PR html_url.
// e.g. "https://github.com/BadgerMaps/badger-go/pull/3636" → "BadgerMaps/badger-go"
// This is more reliable than base.repo.full_name which the search API may omit.
func repoFromPRURL(htmlURL string) string {
	u, err := url.Parse(htmlURL)
	if err != nil {
		return ""
	}
	// Path: "/owner/repo/pull/N" — take first two segments.
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 4)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// ── raw JSON shapes ────────────────────────────────────────────────────────────

type searchItem struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Draft     bool   `json:"draft"`
	HTMLURL   string `json:"html_url"`
	MergedAt  *string `json:"merged_at"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
}

func (it searchItem) toPR() PR {
	state := it.State
	switch {
	case it.MergedAt != nil && *it.MergedAt != "":
		state = "merged"
	case it.Draft:
		state = "draft"
	}
	updatedAt, _ := time.Parse(time.RFC3339, it.UpdatedAt)
	return PR{
		Number:    it.Number,
		Title:     it.Title,
		Author:    it.User.Login,
		Repo:      repoFromPRURL(it.HTMLURL), // parse from html_url — always correct
		Branch:    it.Head.Ref,
		Base:      it.Base.Ref,
		State:     state,
		IsDraft:   it.Draft,
		UpdatedAt: updatedAt,
		URL:       it.HTMLURL,
	}
}

type prDetailRaw struct {
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	State        string  `json:"state"`
	Draft        bool    `json:"draft"`
	Body         string  `json:"body"`
	HTMLURL      string  `json:"html_url"`
	Additions    int     `json:"additions"`
	Deletions    int     `json:"deletions"`
	ChangedFiles int     `json:"changed_files"`
	UpdatedAt    string  `json:"updated_at"`
	MergedAt     *string `json:"merged_at"`
	User         struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
}

func (r prDetailRaw) toPRDetail() *PRDetail {
	state := r.State
	switch {
	case r.MergedAt != nil && *r.MergedAt != "":
		state = "merged"
	case r.Draft:
		state = "draft"
	}
	updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	return &PRDetail{
		PR: PR{
			Number:    r.Number,
			Title:     r.Title,
			Author:    r.User.Login,
			Repo:      r.Base.Repo.FullName, // PR lives in base repo, not head/fork
			Branch:    r.Head.Ref,
			Base:      r.Base.Ref,
			State:     state,
			IsDraft:   r.Draft,
			UpdatedAt: updatedAt,
			URL:       r.HTMLURL,
		},
		Body:         r.Body,
		Additions:    r.Additions,
		Deletions:    r.Deletions,
		ChangedFiles: r.ChangedFiles,
	}
}

type prFileRaw struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}
