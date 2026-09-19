package memory

import (
	"context"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Common typed relations used by fixtures and the compiler ranker.
// Backends may emit other Rel strings; unknown types are treated as "related".
const (
	RelOwns       = "owns"
	RelOwnedBy    = "owned_by"
	RelDecidedBy  = "decided_by"
	RelDecides    = "decides"
	RelBlocks     = "blocks"
	RelBlockedBy  = "blocked_by"
	RelAssignedTo = "assigned_to"
	RelImplements = "implements"
	RelMentions   = "mentions"
	RelMemberOf   = "member_of"
	RelRelated    = "related" // synthesized from untyped Entity.RefIDs only
)

// MaxHops is the compile-time cap on neighbor expansion (runaway guard).
const MaxHops = 3

// Node is a packable graph node (entity).
type Node struct {
	Entity state.Entity
}

// Edge is a typed directed fact between two nodes.
type Edge struct {
	ID         string     `json:"id"`
	FromID     string     `json:"from_id"`
	ToID       string     `json:"to_id"`
	Rel        string     `json:"rel"`
	Fact       string     `json:"fact,omitempty"`
	ValidAt    *time.Time `json:"valid_at,omitempty"`
	InvalidAt  *time.Time `json:"invalid_at,omitempty"`
	EpisodeIDs []string   `json:"episode_ids,omitempty"`
}

// Episode is optional non-lossy source text (provenance). May be empty.
type Episode struct {
	ID      string     `json:"id"`
	Text    string     `json:"text"`
	ValidAt *time.Time `json:"valid_at,omitempty"`
	Source  string     `json:"source,omitempty"`
}

// Neighborhood is a hop-limited slice of the graph.
type Neighborhood struct {
	Nodes    []Node
	Edges    []Edge
	Episodes []Episode
}

// Query is a hybrid seed lookup against a GraphStore.
type Query struct {
	Text     string
	Kinds    []string
	Keywords []string
	Seeds    []string // known entity IDs from the contract
	Limit    int
}

// GraphStore is an optional memory backend: Layer.Load plus hybrid search
// and hop-limited typed neighbor expansion. Compiler owns selection; this
// owns storage. Other backends can implement the same interface later.
type GraphStore interface {
	Layer
	Search(ctx context.Context, handle string, q Query) (Neighborhood, error)
	Neighbors(ctx context.Context, handle string, ids []string, hops int) (Neighborhood, error)
}

// PackText is the budget-pack representation of an edge fact.
func (e Edge) PackText() string {
	fact := e.Fact
	if fact == "" {
		fact = e.FromID + " " + e.Rel + " " + e.ToID
	}
	return "[edge:" + e.Rel + " " + e.FromID + "→" + e.ToID + "] " + fact
}

// PackText is a short provenance snippet.
func (e Episode) PackText() string {
	return "[snippet:" + e.ID + "] " + e.Text
}

// AsGraphStore returns g if layer implements GraphStore.
func AsGraphStore(layer Layer) (GraphStore, bool) {
	g, ok := layer.(GraphStore)
	return g, ok
}
