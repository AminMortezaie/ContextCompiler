package fixture

import "github.com/aminmortezaie/contextcompiler/internal/state"

// EdgeSpec is a typed org-state edge for fixtures (compiler neighborhood).
// Relations are causal / ownership / action — not co-occurrence soup.
type EdgeSpec struct {
	From string
	To   string
	Rel  string
	Fact string
	// EpisodeIDs optionally cite a source snippet (provenance).
	EpisodeIDs []string
}

// EpisodeSpec is optional non-lossy source text attached to edges.
type EpisodeSpec struct {
	ID     string
	Text   string
	Source string
}

// Day0Edges returns the typed Project X dependency graph.
// IDs match Day0() entities. This replaces hardcoded hub boosts.
func Day0Edges() []EdgeSpec {
	const standup = "ep-standup-2026-09-10"
	return []EdgeSpec{
		// Ownership
		{From: "usr-sarah", To: "proj-x", Rel: "owns", Fact: "Sarah Chen (VP Engineering) owns cross-team delivery decisions for Project X."},
		{From: "usr-priya", To: "proj-x", Rel: "owns", Fact: "Priya Nair (PM) owns Product for Project X."},
		{From: "team-backend", To: "tkt-042", Rel: "owns", Fact: "Backend team owns the auth service contract freeze ticket."},
		{From: "usr-marcus", To: "task-freeze", Rel: "owns", Fact: "Marcus Lee owns implementing the auth contract freeze."},

		// Membership
		{From: "usr-marcus", To: "team-backend", Rel: "member_of", Fact: "Marcus Lee is Backend Tech Lead."},
		{From: "usr-alex", To: "team-frontend", Rel: "member_of", Fact: "Alex Kim is on the frontend team."},

		// Decision / causal
		{From: "dec-001", To: "usr-sarah", Rel: "decided_by", Fact: "Decision dec-001 was approved by Sarah Chen."},
		{From: "dec-001", To: "proj-x", Rel: "decides", Fact: "dec-001 accepts a two-sprint delay on Project X to absorb API redesign scope creep."},
		{From: "tkt-042", To: "dec-001", Rel: "implements", Fact: "Ticket tkt-042 implements the auth freeze required by dec-001."},
		{From: "task-freeze", To: "tkt-042", Rel: "implements", Fact: "Task-freeze implements ticket tkt-042."},
		{From: "task-freeze", To: "dec-001", Rel: "implements", Fact: "Task-freeze carries out dec-001 for the backend."},

		// Blocking
		{From: "tkt-042", To: "proj-x", Rel: "blocks", Fact: "Ticket tkt-042 blocks Project X until the API redesign is signed off."},
		{From: "tkt-042", To: "usr-marcus", Rel: "assigned_to", Fact: "tkt-042 is assigned to Marcus Lee."},

		// Evidence mentions (conversation → specific nodes, not a fully-connected clique)
		{From: "conv-standup", To: "tkt-042", Rel: "mentions", Fact: "Standup: Marcus still blocked on auth freeze tkt-042.", EpisodeIDs: []string{standup}},
		{From: "conv-standup", To: "dec-001", Rel: "mentions", Fact: "Standup: Sarah confirmed delay decision dec-001 stands.", EpisodeIDs: []string{standup}},
		{From: "conv-standup", To: "usr-marcus", Rel: "mentions", Fact: "Standup mentions Marcus on the freeze.", EpisodeIDs: []string{standup}},
		{From: "conv-standup", To: "usr-sarah", Rel: "mentions", Fact: "Standup mentions Sarah confirming the decision.", EpisodeIDs: []string{standup}},
		{From: "evt-kickoff", To: "proj-x", Rel: "mentions", Fact: "Kickoff meeting was for Project X SSO (before the delay)."},

		// Unrelated workstream — typed, disconnected from Project X
		{From: "tkt-099", To: "usr-alex", Rel: "assigned_to", Fact: "Dashboard color ticket assigned to Alex Kim."},
		{From: "tkt-099", To: "proj-y", Rel: "implements", Fact: "tkt-099 is marketing-site work on Project Y."},
		{From: "task-colors", To: "tkt-099", Rel: "implements", Fact: "Brand-color task implements tkt-099."},
	}
}

// Day0Episodes returns optional provenance snippets for Day-0 edges.
func Day0Episodes() []EpisodeSpec {
	return []EpisodeSpec{{
		ID:     "ep-standup-2026-09-10",
		Text:   "Marcus: still blocked on auth freeze (tkt-042). Sarah confirmed delay decision dec-001 stands. Priya asked for ETA after redesign.",
		Source: "conv-standup",
	}}
}

// GraphSpecs returns Day-0 typed edges that exist in entities, plus typed
// local clusters among noise entities (assigned_to / owns / implements / blocks).
// Noise clusters do not connect to the Project X subgraph.
func GraphSpecs(entities []state.Entity) []EdgeSpec {
	have := make(map[string]bool, len(entities))
	for _, e := range entities {
		have[e.ID] = true
	}
	var out []EdgeSpec
	for _, s := range Day0Edges() {
		if have[s.From] && have[s.To] {
			out = append(out, s)
		}
	}
	out = append(out, noiseTypedEdges(entities)...)
	return out
}

// noiseTypedEdges builds small typed clusters among synthetic noise entities.
// Cluster i: user owns project; decision decided_by user and decides project;
// ticket assigned_to user and blocks project; task implements ticket.
func noiseTypedEdges(entities []state.Entity) []EdgeSpec {
	var users, tickets, decisions, tasks, projects []state.Entity
	for _, e := range entities {
		if e.Meta == nil || e.Meta["noise"] != "true" {
			continue
		}
		switch e.Kind {
		case state.KindUser:
			users = append(users, e)
		case state.KindTicket:
			tickets = append(tickets, e)
		case state.KindDecision:
			decisions = append(decisions, e)
		case state.KindTask:
			tasks = append(tasks, e)
		case state.KindProject:
			projects = append(projects, e)
		}
	}
	n := len(users)
	for _, s := range []int{len(tickets), len(decisions), len(tasks), len(projects)} {
		if s < n {
			n = s
		}
	}
	out := make([]EdgeSpec, 0, n*6)
	for i := 0; i < n; i++ {
		u, p, d, t, k := users[i], projects[i], decisions[i], tickets[i], tasks[i]
		out = append(out,
			EdgeSpec{From: u.ID, To: p.ID, Rel: "owns", Fact: u.Title + " owns " + p.Title + "."},
			EdgeSpec{From: d.ID, To: u.ID, Rel: "decided_by", Fact: d.Title + " decided_by " + u.Title + "."},
			EdgeSpec{From: d.ID, To: p.ID, Rel: "decides", Fact: d.Title + " decides status of " + p.Title + "."},
			EdgeSpec{From: t.ID, To: u.ID, Rel: "assigned_to", Fact: t.Title + " assigned_to " + u.Title + "."},
			EdgeSpec{From: t.ID, To: p.ID, Rel: "blocks", Fact: t.Title + " blocks " + p.Title + "."},
			EdgeSpec{From: k.ID, To: t.ID, Rel: "implements", Fact: k.Title + " implements " + t.Title + "."},
		)
	}
	return out
}
