package fixture

// Day0TaskSuite returns deterministic narrative tasks over the Day-0 org state.
// All tasks share the same underlying store from Day0() / Scale(...).
func Day0TaskSuite() []Task {
	return []Task{
		projXDelayTask(),
		causalTask(),
		decisionTask(),
		ownershipTask(),
		actionTask(),
	}
}

func projXDelayTask() Task {
	return Task{
		ID:       "task-proj-x-delay",
		Question: "Why was Project X delayed, who made the relevant decision, and what action should the backend team take?",
		RelevantIDs: []string{
			"proj-x", "dec-001", "tkt-042", "usr-sarah", "usr-marcus",
			"conv-standup", "task-freeze", "team-backend",
		},
		RequiredPhrases: []string{"API redesign", "Sarah Chen", "backend"},
	}
}

func causalTask() Task {
	return Task{
		ID:       "task-causal",
		Question: "What caused the Project X schedule slip?",
		RelevantIDs: []string{
			"proj-x", "conv-standup", "dec-001",
		},
		RequiredPhrases: []string{"API redesign", "scope creep"},
	}
}

func decisionTask() Task {
	return Task{
		ID:       "task-decision",
		Question: "Who approved accepting the Project X delay, and what is the decision ID?",
		RelevantIDs: []string{
			"dec-001", "usr-sarah", "conv-standup", "proj-x",
		},
		RequiredPhrases: []string{"Sarah Chen", "dec-001"},
	}
}

func ownershipTask() Task {
	return Task{
		ID:       "task-ownership",
		Question: "Who owns implementing the auth service contract freeze for Project X?",
		RelevantIDs: []string{
			"tkt-042", "usr-marcus", "task-freeze", "team-backend",
		},
		RequiredPhrases: []string{"Marcus Lee", "tkt-042"},
	}
}

func actionTask() Task {
	return Task{
		ID:       "task-action",
		Question: "What prioritized action should the backend team take on Project X right now?",
		RelevantIDs: []string{
			"tkt-042", "task-freeze", "dec-001", "usr-marcus",
		},
		RequiredPhrases: []string{"freeze", "auth", "backend"},
	}
}
