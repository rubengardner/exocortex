package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Issue represents a single Jira issue.
type Issue struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
	URL      string
}

// Client is a minimal Jira REST API v3 client using Basic Auth.
type Client struct {
	baseURL  string
	email    string
	apiToken string
}

// New returns a new Client.
func New(baseURL, email, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		apiToken: apiToken,
	}
}

// FetchBoard fetches issues for the given statuses, returning a map from
// status name to the issues in that status bucket.
//
// If boardID > 0 the Agile board endpoint is used
// (GET /rest/agile/1.0/board/{id}/issue), which restricts results to that
// specific board. Otherwise the project-wide search endpoint is used.
//
// teamID, when non-empty, restricts results to issues belonging to that Jira
// team (matched via customfield_10001).
func (c *Client) FetchBoard(boardID int, project string, statuses []string, teamID string) (map[string][]Issue, error) {
	quoted := make([]string, len(statuses))
	for i, s := range statuses {
		quoted[i] = fmt.Sprintf("%q", s)
	}

	var endpoint, jql string
	if boardID > 0 {
		// Use the Agile board endpoint — no status filter in JQL so we never
		// send status names that might not match the remote values. Issues are
		// grouped client-side by their actual status.name, then only the
		// configured statuses are displayed as columns.
		endpoint = fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue", c.baseURL, boardID)
		if teamID != "" {
			jql = fmt.Sprintf(`cf[10001]=%q ORDER BY updated DESC`, teamID)
		} else {
			jql = "ORDER BY updated DESC"
		}
	} else {
		endpoint = c.baseURL + "/rest/api/3/search/jql"
		jql = fmt.Sprintf(`project=%s AND status in (%s)`, project, strings.Join(quoted, ","))
		if teamID != "" {
			jql += fmt.Sprintf(` AND cf[10001]=%q`, teamID)
		}
		jql += " ORDER BY updated DESC"
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: create request: %w", err)
	}

	q := url.Values{}
	q.Set("jql", jql)
	q.Set("fields", "summary,status,assignee")
	q.Set("maxResults", "50")
	req.URL.RawQuery = q.Encode()

	creds := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
				Assignee *struct {
					DisplayName string `json:"displayName"`
				} `json:"assignee"`
			} `json:"fields"`
		} `json:"issues"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: decode response: %w", err)
	}

	// Pre-populate every requested status bucket (including empties).
	board := make(map[string][]Issue, len(statuses))
	for _, s := range statuses {
		board[s] = nil
	}

	for _, raw := range result.Issues {
		assignee := ""
		if raw.Fields.Assignee != nil {
			assignee = raw.Fields.Assignee.DisplayName
		}
		issue := Issue{
			Key:      raw.Key,
			Summary:  raw.Fields.Summary,
			Status:   raw.Fields.Status.Name,
			Assignee: assignee,
			URL:      issueURL(c.baseURL, raw.Key),
		}
		board[issue.Status] = append(board[issue.Status], issue)
	}

	return board, nil
}

func issueURL(baseURL, key string) string {
	return baseURL + "/browse/" + key
}

// FetchIssueDescription fetches the description of a single issue and returns
// it as Markdown text converted from Atlassian Document Format (ADF).
func (c *Client) FetchIssueDescription(key string) (string, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/rest/api/3/issue/%s?fields=description", c.baseURL, key), nil)
	if err != nil {
		return "", fmt.Errorf("jira: create request: %w", err)
	}

	creds := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.apiToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Fields struct {
			Description *adfNode `json:"description"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("jira: decode response: %w", err)
	}

	if result.Fields.Description == nil {
		return "_No description provided._\n", nil
	}
	return adfToMarkdown(*result.Fields.Description), nil
}

// --- ADF → Markdown ----------------------------------------------------------

// adfNode is a node in Atlassian Document Format JSON.
type adfNode struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Content []adfNode              `json:"content,omitempty"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Marks   []adfMark              `json:"marks,omitempty"`
}

type adfMark struct {
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs,omitempty"`
}

func adfToMarkdown(root adfNode) string {
	return strings.TrimSpace(renderADF(root, 0)) + "\n"
}

func renderADF(node adfNode, depth int) string {
	switch node.Type {
	case "doc":
		var sb strings.Builder
		for _, child := range node.Content {
			sb.WriteString(renderADF(child, depth))
		}
		return sb.String()

	case "paragraph":
		var sb strings.Builder
		for _, child := range node.Content {
			sb.WriteString(renderADF(child, depth))
		}
		return sb.String() + "\n\n"

	case "text":
		t := node.Text
		for _, m := range node.Marks {
			switch m.Type {
			case "strong":
				t = "**" + t + "**"
			case "em":
				t = "_" + t + "_"
			case "code":
				t = "`" + t + "`"
			case "strike":
				t = "~~" + t + "~~"
			case "link":
				href := ""
				if m.Attrs != nil {
					if h, ok := m.Attrs["href"].(string); ok {
						href = h
					}
				}
				t = "[" + t + "](" + href + ")"
			}
		}
		return t

	case "heading":
		level := 1
		if node.Attrs != nil {
			if l, ok := node.Attrs["level"].(float64); ok {
				level = int(l)
			}
		}
		var sb strings.Builder
		for _, child := range node.Content {
			sb.WriteString(renderADF(child, depth))
		}
		return strings.Repeat("#", level) + " " + sb.String() + "\n\n"

	case "bulletList":
		var sb strings.Builder
		for _, item := range node.Content {
			sb.WriteString(renderListItem(item, "-", depth))
		}
		return sb.String() + "\n"

	case "orderedList":
		var sb strings.Builder
		for i, item := range node.Content {
			sb.WriteString(renderListItem(item, fmt.Sprintf("%d.", i+1), depth))
		}
		return sb.String() + "\n"

	case "codeBlock":
		lang := ""
		if node.Attrs != nil {
			if l, ok := node.Attrs["language"].(string); ok {
				lang = l
			}
		}
		var sb strings.Builder
		for _, child := range node.Content {
			sb.WriteString(renderADF(child, depth))
		}
		return "```" + lang + "\n" + sb.String() + "```\n\n"

	case "blockquote":
		var sb strings.Builder
		for _, child := range node.Content {
			content := strings.TrimRight(renderADF(child, depth), "\n")
			for _, line := range strings.Split(content, "\n") {
				sb.WriteString("> " + line + "\n")
			}
		}
		return sb.String() + "\n"

	case "hardBreak":
		return "  \n"

	case "rule":
		return "---\n\n"

	case "mention":
		if node.Attrs != nil {
			if t, ok := node.Attrs["text"].(string); ok {
				return t
			}
		}
		return "@mention"

	case "inlineCard", "blockCard":
		if node.Attrs != nil {
			if u, ok := node.Attrs["url"].(string); ok {
				return u
			}
		}
		return ""

	default:
		var sb strings.Builder
		for _, child := range node.Content {
			sb.WriteString(renderADF(child, depth))
		}
		if node.Text != "" {
			sb.WriteString(node.Text)
		}
		return sb.String()
	}
}

func renderListItem(item adfNode, bullet string, depth int) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder
	first := true
	for _, child := range item.Content {
		switch child.Type {
		case "bulletList":
			for _, subItem := range child.Content {
				sb.WriteString(renderListItem(subItem, "-", depth+1))
			}
		case "orderedList":
			for i, subItem := range child.Content {
				sb.WriteString(renderListItem(subItem, fmt.Sprintf("%d.", i+1), depth+1))
			}
		default:
			content := strings.TrimRight(renderADF(child, depth), "\n")
			if first {
				sb.WriteString(indent + bullet + " " + content + "\n")
				first = false
			} else {
				for _, line := range strings.Split(content, "\n") {
					if line != "" {
						sb.WriteString(indent + "  " + line + "\n")
					}
				}
			}
		}
	}
	return sb.String()
}
