package ui

// ViewState is the active UI layer. Exported so tests can inspect it.
type ViewState int

// viewState is an internal alias kept for brevity in switch statements.
type viewState = ViewState

const (
	StateList          ViewState = iota // main nucleus list
	StateConfirmDelete                  // delete confirmation dialog
	StateHelp                           // full-page keyboard shortcuts
	StateJiraBoard                      // live Jira kanban view
	StateJiraDetail                     // single-issue description overlay
	StateNucleusDetail                  // full-screen 3-panel nucleus dashboard
	StateNeuronAdd                      // neuron type picker overlay (from detail)
	StateGitHubView                     // GitHub PR list view
	StateGitHubPRDetail                 // full-screen GitHub PR detail
	StateGitHubFilter                   // filter modal overlay on top of StateGitHubView
	StateNucleusModal                   // unified new-nucleus modal (from any screen)
	StatePRAdd                          // PR add overlay (from nucleus detail)

	// Deprecated aliases kept for test compatibility — map to StateNucleusModal.
	StateNewOverlay    = StateNucleusModal
	StateRepoSelect    = StateNucleusModal
	StateProfileSelect = StateNucleusModal
	StateBranchSearch  = StateNucleusModal
)

// Internal aliases — allow the rest of the package to use lowercase names
// in switch statements for brevity.
const (
	stateList          = StateList
	stateConfirmDelete = StateConfirmDelete
	stateHelp          = StateHelp
	stateJiraBoard     = StateJiraBoard
	stateJiraDetail    = StateJiraDetail
	stateNucleusDetail  = StateNucleusDetail
	stateNeuronAdd      = StateNeuronAdd
	stateGitHubView     = StateGitHubView
	stateGitHubPRDetail = StateGitHubPRDetail
	stateGitHubFilter   = StateGitHubFilter
	stateNucleusModal   = StateNucleusModal
	statePRAdd          = StatePRAdd
)
