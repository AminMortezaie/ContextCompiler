// Package fixture provides synthetic org state and the Project X golden task.
// Day0 is the tiny core narrative; Scale helpers grow it to larger token rungs with noise.
package fixture

import "github.com/aminmortezaie/contextcompiler/internal/state"

// Task is a scored evaluation task with golden relevant entity IDs and answer checks.
type Task struct {
	ID              string
	Question        string
	RelevantIDs     []string // golden relevant state entity IDs
	RequiredPhrases []string // substrings that must appear in a successful answer
}

// Day0 returns the small company fixture and the Project X delay golden task.
// No Postgres required.
func Day0() (*state.Store, Task) {
	entities := []state.Entity{
		{
			ID: "co-001", Kind: state.KindCompany, Title: "Acme Systems",
			Text: "Acme Systems builds B2B collaboration software. HQ in Yerevan. ~120 employees.",
		},
		{
			ID: "usr-sarah", Kind: state.KindUser, Title: "Sarah Chen",
			Text: "Sarah Chen is VP Engineering. She owns cross-team delivery decisions for Project X.",
			Meta: map[string]string{"role": "VP Engineering"},
		},
		{
			ID: "usr-marcus", Kind: state.KindUser, Title: "Marcus Lee",
			Text: "Marcus Lee is Backend Tech Lead on Project X auth service.",
			Meta: map[string]string{"role": "Backend Tech Lead", "team": "team-backend"},
		},
		{
			ID: "usr-priya", Kind: state.KindUser, Title: "Priya Nair",
			Text: "Priya Nair is Product Manager for Project X.",
			Meta: map[string]string{"role": "PM"},
		},
		{
			ID: "usr-alex", Kind: state.KindUser, Title: "Alex Kim",
			Text: "Alex Kim is Frontend Engineer on the dashboard rewrite (unrelated to Project X delay).",
			Meta: map[string]string{"role": "Frontend Engineer"},
		},
		{
			ID: "team-backend", Kind: state.KindTeam, Title: "Backend Team",
			Text:   "Backend team owns auth, API gateway, and billing services. Lead: Marcus Lee.",
			RefIDs: []string{"usr-marcus"},
		},
		{
			ID: "team-frontend", Kind: state.KindTeam, Title: "Frontend Team",
			Text:   "Frontend team owns web dashboard. Lead: Alex Kim. Not involved in Project X delay.",
			RefIDs: []string{"usr-alex"},
		},
		{
			ID: "proj-x", Kind: state.KindProject, Title: "Project X",
			Text:   "Project X: unified auth and SSO rollout. Status: DELAYED. Original ship date slipped two sprints due to scope creep from API redesign.",
			RefIDs: []string{"usr-sarah", "usr-marcus", "usr-priya", "dec-001", "tkt-042"},
			Meta:   map[string]string{"status": "delayed"},
		},
		{
			ID: "proj-y", Kind: state.KindProject, Title: "Project Y",
			Text: "Project Y: marketing site refresh. On track. Unrelated to Project X.",
			Meta: map[string]string{"status": "on_track"},
		},
		{
			ID: "dec-001", Kind: state.KindDecision, Title: "Accept Project X delay",
			Text:   "Decision dec-001: Sarah Chen (VP Engineering) approved accepting a two-sprint delay on Project X to absorb scope creep from the API redesign. Backend must freeze the auth service contract until redesign sign-off.",
			RefIDs: []string{"proj-x", "usr-sarah", "tkt-042"},
		},
		{
			ID: "tkt-042", Kind: state.KindTicket, Title: "Auth service contract freeze",
			Text:   "Ticket tkt-042: Backend freeze the auth service API contract. Assigned to Marcus Lee. Blocking Project X until API redesign is signed off. Priority: P1.",
			RefIDs: []string{"proj-x", "usr-marcus", "dec-001"},
		},
		{
			ID: "tkt-099", Kind: state.KindTicket, Title: "Dashboard button color",
			Text:   "Ticket tkt-099: Change primary button color on marketing site. Unrelated noise for Project X delay questions.",
			RefIDs: []string{"proj-y", "usr-alex"},
		},
		{
			ID: "doc-roadmap", Kind: state.KindDocument, Title: "Q3 Engineering Roadmap",
			Text: "Q3 roadmap mentions Project X SSO and Project Y marketing refresh. Generic planning doc with little delay detail.",
		},
		{
			ID: "doc-handbook", Kind: state.KindDocument, Title: "Employee Handbook",
			Text: "Company PTO policy, remote work guidelines, and expense reimbursement. Irrelevant to Project X delay.",
		},
		{
			ID: "conv-standup", Kind: state.KindConversation, Title: "Backend standup 2026-09-10",
			Text:   "Marcus: still blocked on auth freeze (tkt-042). Sarah confirmed delay decision dec-001 stands. Priya asked for ETA after redesign.",
			RefIDs: []string{"tkt-042", "dec-001", "usr-marcus", "usr-sarah", "usr-priya"},
		},
		{
			ID: "conv-random", Kind: state.KindConversation, Title: "Random #coffee",
			Text:   "Alex: anyone try the new espresso machine? Unrelated chatter.",
			RefIDs: []string{"usr-alex"},
		},
		{
			ID: "task-freeze", Kind: state.KindTask, Title: "Implement auth contract freeze",
			Text:   "Task: Marcus implements auth service contract freeze per tkt-042 after dec-001.",
			RefIDs: []string{"tkt-042", "usr-marcus", "dec-001"},
		},
		{
			ID: "task-colors", Kind: state.KindTask, Title: "Update brand colors",
			Text:   "Task: Alex updates CSS variables for Project Y. Irrelevant.",
			RefIDs: []string{"tkt-099", "usr-alex"},
		},
		{
			ID: "evt-kickoff", Kind: state.KindEvent, Title: "Project X kickoff",
			Text:   "Kickoff meeting for Project X SSO. Happened before the delay.",
			RefIDs: []string{"proj-x"},
		},
		{
			ID: "evt-party", Kind: state.KindEvent, Title: "Summer picnic",
			Text: "Company picnic. Completely irrelevant to engineering delays.",
		},
	}

	task := projXDelayTask()

	return state.NewStore(entities), task
}
