package permissions

import (
	"fmt"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Policy is a simple allow/deny stub over entity kinds and tags (Meta["tag"] or Meta["tags"]).
type Policy struct {
	AllowKinds []string `json:"allow_kinds,omitempty"`
	DenyKinds  []string `json:"deny_kinds,omitempty"`
	AllowTags  []string `json:"allow_tags,omitempty"`
	DenyTags   []string `json:"deny_tags,omitempty"`
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
	allowTag := toSet(p.AllowTags)
	denyTag := toSet(p.DenyTags)

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
		tags := entityTags(e)
		if len(allowTag) > 0 && !hasAnyTag(tags, allowTag) {
			denials = append(denials, Denial{
				ID: e.ID, Reason: "permissions: no allowed tag match",
			})
			continue
		}
		if hasAnyTag(tags, denyTag) {
			denials = append(denials, Denial{
				ID: e.ID, Reason: "permissions: denied tag match",
			})
			continue
		}
		allowed = append(allowed, e)
	}
	return allowed, denials
}

func (p Policy) isEmpty() bool {
	return len(p.AllowKinds) == 0 && len(p.DenyKinds) == 0 &&
		len(p.AllowTags) == 0 && len(p.DenyTags) == 0
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

func entityTags(e state.Entity) []string {
	if e.Meta == nil {
		return nil
	}
	var tags []string
	if t, ok := e.Meta["tag"]; ok && t != "" {
		tags = append(tags, strings.ToLower(t))
	}
	if ts, ok := e.Meta["tags"]; ok && ts != "" {
		for _, part := range strings.Split(ts, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part != "" {
				tags = append(tags, part)
			}
		}
	}
	return tags
}

func hasAnyTag(entityTags []string, denyOrAllow map[string]bool) bool {
	if len(denyOrAllow) == 0 {
		return false
	}
	for _, t := range entityTags {
		if denyOrAllow[t] {
			return true
		}
	}
	return false
}
