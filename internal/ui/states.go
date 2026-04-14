package ui

// ViewState is the active UI layer. Exported so tests can inspect it.
type ViewState int

// viewState is an internal alias kept for brevity in switch statements.
type viewState = ViewState

const (
	StateList          ViewState = iota // main nucleus list
	StateNewOverlay                     // new-nucleus form overlay
	StateConfirmDelete                  // delete confirmation dialog
	StateHelp                           // full-page keyboard shortcuts
	StateRepoSelect                     // repo picker (before new-nucleus form)
	StateProfileSelect                  // profile picker (after repo, before form)
	StateJiraBoard                      // live Jira kanban view
	StateJiraDetail                     // single-issue description overlay
	StateNucleusDetail                  // full-screen 3-panel nucleus dashboard
	StateNeuronAdd                      // neuron type picker overlay (from detail)
)

// Internal aliases — allow the rest of the package to use lowercase names
// in switch statements for brevity.
const (
	stateList          = StateList
	stateNewOverlay    = StateNewOverlay
	stateConfirmDelete = StateConfirmDelete
	stateHelp          = StateHelp
	stateRepoSelect    = StateRepoSelect
	stateProfileSelect = StateProfileSelect
	stateJiraBoard     = StateJiraBoard
	stateJiraDetail    = StateJiraDetail
	stateNucleusDetail = StateNucleusDetail
	stateNeuronAdd     = StateNeuronAdd
)
