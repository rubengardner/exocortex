package github

import "strings"

// PRFilter holds the active filter criteria for the GitHub PR list.
// Zero value means "no filter" (falls back to involves:@me behaviour).
type PRFilter struct {
	// Authors is the set of GitHub logins to include.
	// Empty = no author filter (uses involves:@me instead).
	// Special sentinel "!me" means "everyone except the authenticated user".
	Authors []string

	// Repos is the set of "owner/repo" strings to include.
	// Empty = no repo filter. Applied client-side after the API response.
	Repos []string
}

// IsZero reports whether no filter criteria are set.
func (f PRFilter) IsZero() bool {
	return len(f.Authors) == 0 && len(f.Repos) == 0
}

// BuildQuery returns the GitHub search query string for the given filter.
// myLogin is the authenticated user's login (GitHubConfig.MyLogin); it is used
// to expand the "!me" sentinel into -author:<login>.
// org is an optional org scope appended to every query.
func BuildQuery(myLogin, org string, f PRFilter) string {
	var parts []string
	parts = append(parts, "is:pr", "is:open")

	// Author fragment.
	if len(f.Authors) == 0 {
		parts = append(parts, "involves:@me")
	} else {
		for _, a := range f.Authors {
			if a == "!me" {
				if myLogin != "" {
					parts = append(parts, "-author:"+myLogin)
				}
			} else {
				parts = append(parts, "author:"+a)
			}
		}
	}

	if org != "" {
		parts = append(parts, "org:"+org)
	}

	return strings.Join(parts, " ")
}

// ApplyRepoFilter returns only those PRs whose Repo field appears in repos.
// If repos is empty the original slice is returned unchanged.
func ApplyRepoFilter(prs []PR, repos []string) []PR {
	if len(repos) == 0 {
		return prs
	}
	set := make(map[string]struct{}, len(repos))
	for _, r := range repos {
		set[r] = struct{}{}
	}
	out := prs[:0:0] // reuse backing array where possible
	for _, pr := range prs {
		if _, ok := set[pr.Repo]; ok {
			out = append(out, pr)
		}
	}
	return out
}
