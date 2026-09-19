package permissions

import (
	"fmt"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Policy is a simple allow/deny stub over entity kinds.
type Policy struct {
	AllowKinds []string `json:"allow_kinds,omitempty"`
	DenyKinds  []string `json:"deny_kinds,omitempty"`
}

// Denial records an entity excluded by permissions before compile ranking.
type Denial struct {
	ID     string
	Reason string
}

// Filter returns entities permitted by the policy and denials for blocked entities.
func Filter(entities []state.Entity, p Policy) (allowed []state.Entity, denials []Denial) {
	if p.isEmpty() {
		return entities, nil
	}
	allowKind := toSet(p.AllowKinds)
	denyKind := toSet(p.DenyKinds)

	for _, e := range entities {
		kind := string(e.Kind)
		if len(allowKind) > 0 && !allowKind[kind] {
			denials = append(denials, Denial{
				ID: e.ID, Reason: fmt.Sprintf("permissions: kind %q not in allow_kinds", kind),
			})
			continue
		}
		if denyKind[kind] {
			denials = append(denials, Denial{
				ID: e.ID, Reason: fmt.Sprintf("permissions: kind %q denied", kind),
			})
			continue
		}
		allowed = append(allowed, e)
	}
	return allowed, denials
}

func (p Policy) isEmpty() bool {
	return len(p.AllowKinds) == 0 && len(p.DenyKinds) == 0
}

func toSet(vals []string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			m[v] = true
		}
	}
	return m
}
