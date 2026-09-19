package fixture

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// DefaultSeed is the deterministic RNG seed for the large synthetic org.
const DefaultSeed = 42

// TargetTokens100K is the Phase-1 bakeoff org-state scale (~100K tokens, chars/4).
const TargetTokens100K = 100_000

// TargetTokens500K is the next scale rung (~500K tokens, chars/4 estimator).
const TargetTokens500K = 500_000

// TargetTokens1M is the next scale rung (~1M tokens, chars/4 estimator).
const TargetTokens1M = 1_000_000

// TargetTokens5M is the next scale rung (~5M tokens, chars/4 estimator).
const TargetTokens5M = 5_000_000

// Scale returns a synthetic org whose packed entity text is approximately
// targetTokens (chars/4 estimator), seeded for reproducibility.
// It always includes the Day-0 Project X golden narrative entities and
// dilutes them with irrelevant org noise (noise text deliberately avoids
// golden keywords so retrieval arms can discriminate).
func Scale(targetTokens int, seed int64) (*state.Store, Task) {
	if targetTokens <= 0 {
		targetTokens = TargetTokens100K
	}
	base, task := Day0()
	entities := append([]state.Entity{}, base.All()...)

	rng := rand.New(rand.NewSource(seed))
	// Keep generation linear in the number of entities. Re-estimating the full
	// slice on every append made the 1M rung needlessly quadratic.
	current := estimateEntities(entities)

	noiseKinds := []state.EntityKind{
		state.KindUser, state.KindProject, state.KindTeam, state.KindDocument,
		state.KindConversation, state.KindTicket, state.KindDecision,
		state.KindTask, state.KindEvent,
	}
	topics := []string{
		"marketing campaign Q4", "office relocation", "payroll system migration",
		"design system tokens", "customer webinar series", "security awareness training",
		"laptop refresh cycle", "benefits open enrollment", "sales CRM cleanup",
		"partner portal branding", "data warehouse ETL", "mobile app icon redesign",
		"cafeteria menu survey", "intern onboarding checklist", "legal NDA template",
		"cloud cost optimization", "incident severity glossary", "holiday calendar sync",
	}

	idx := 0
	for current < targetTokens {
		kind := noiseKinds[rng.Intn(len(noiseKinds))]
		topic := topics[rng.Intn(len(topics))]
		id := fmt.Sprintf("noise-%s-%05d", kind, idx)
		// Opaque title: avoid echoing kind names that overlap the golden query ("decision", "project", ...).
		title := fmt.Sprintf("Ops note %05d — %s", idx, topic)
		need := targetTokens - current
		padTokens := 80 + rng.Intn(120)
		if need < padTokens {
			padTokens = need
		}
		if padTokens < 20 {
			padTokens = 20
		}
		e := state.Entity{
			ID:    id,
			Kind:  kind,
			Title: title,
			Text:  noiseBody(rng, topic, padTokens),
			Meta:  map[string]string{"noise": "true", "topic": topic},
		}
		entities = append(entities, e)
		current += tokens.Estimate(e.PackText())
		idx++
		if idx > 50_000 {
			break
		}
	}

	return state.NewStore(entities), task
}

// EstimateStoreTokens returns chars/4 estimate of all packed entity texts.
func EstimateStoreTokens(st *state.Store) int {
	return estimateEntities(st.All())
}

func estimateEntities(ents []state.Entity) int {
	n := 0
	for _, e := range ents {
		n += tokens.Estimate(e.PackText())
	}
	return n
}

func noiseBody(rng *rand.Rand, topic string, approxTokens int) string {
	targetChars := approxTokens * tokens.CharsPerToken
	var b strings.Builder
	b.Grow(targetChars)
	// IMPORTANT: do not mention Project X, SSO, auth freeze, API redesign, tkt-042,
	// Sarah Chen, etc. — those are golden retrieval signals.
	b.WriteString(fmt.Sprintf(
		"This record concerns %s. It has no bearing on engineering delivery delays for other workstreams. ",
		topic,
	))
	fillers := []string{
		"Stakeholders reviewed status in a routine sync without actionable outcomes for other teams. ",
		"Budget line items remain within the approved quarterly envelope for non-critical work. ",
		"No cross-team dependency on blocked services was identified during the latest check-in. ",
		"Documentation was updated for internal wiki readers; external customers are unaffected. ",
		"Follow-ups were parked for the next planning cycle pending capacity from adjacent teams. ",
		"Metrics dashboards show stable trends with seasonal variation typical for this workstream. ",
		"Cross-functional partners acknowledged the update and requested no immediate escalation. ",
		"Historical notes reference prior fiscal year initiatives that have since been archived. ",
		"Owner confirmed the checklist items are complete and ready for archival in the shared drive. ",
		"Scheduling conflicts were resolved by moving the review to the following Wednesday morning. ",
	}
	for b.Len() < targetChars {
		b.WriteString(fillers[rng.Intn(len(fillers))])
	}
	return b.String()
}
