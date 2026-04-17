package ui

// ProfilesLoadedMsg returns a profilesLoadedMsg for use in external test files.
func ProfilesLoadedMsg(names []string) interface{} {
	paths := make(map[string]string)
	for _, n := range names {
		paths[n] = "/path/" + n
	}
	return profilesLoadedMsg{names: names, paths: paths}
}
