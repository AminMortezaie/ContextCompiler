// Package state defines organizational entity types used as context sources.
package state

// EntityKind identifies the type of organizational state.
type EntityKind string

const (
	KindCompany      EntityKind = "company"
	KindUser         EntityKind = "user"
	KindProject      EntityKind = "project"
	KindTeam         EntityKind = "team"
	KindDocument     EntityKind = "document"
	KindConversation EntityKind = "conversation"
	KindTicket       EntityKind = "ticket"
	KindDecision     EntityKind = "decision"
	KindTask         EntityKind = "task"
	KindEvent        EntityKind = "event"
)

// Entity is a single piece of organizational state that can be packed into context.
type Entity struct {
	ID      string     `json:"id"`
	Kind    EntityKind `json:"kind"`
	Title   string     `json:"title"`
	Text    string     `json:"text"`
	RefIDs  []string   `json:"ref_ids,omitempty"` // related entity IDs
	Meta    map[string]string `json:"meta,omitempty"`
}

// Store is an in-memory collection of entities indexed by ID.
type Store struct {
	Entities []Entity
	byID     map[string]Entity
}

// NewStore builds a Store from a slice of entities.
func NewStore(entities []Entity) *Store {
	s := &Store{Entities: entities, byID: make(map[string]Entity, len(entities))}
	for _, e := range entities {
		s.byID[e.ID] = e
	}
	return s
}

// Get returns an entity by ID.
func (s *Store) Get(id string) (Entity, bool) {
	e, ok := s.byID[id]
	return e, ok
}

// All returns all entities in insertion order.
func (s *Store) All() []Entity {
	return s.Entities
}

// IDs returns all entity IDs.
func (s *Store) IDs() []string {
	ids := make([]string, len(s.Entities))
	for i, e := range s.Entities {
		ids[i] = e.ID
	}
	return ids
}

// PackText returns a deterministic text representation for packing into a prompt.
func (e Entity) PackText() string {
	return "[" + string(e.Kind) + ":" + e.ID + "] " + e.Title + "\n" + e.Text
}
