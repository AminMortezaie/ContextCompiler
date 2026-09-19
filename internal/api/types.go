package api

import (
	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// CompileRequest is the HTTP body for POST /v1/compile.
type CompileRequest struct {
	TaskContract compiler.TaskContract `json:"task_contract"`
	State        StateRef              `json:"state"`
	Budget       BudgetConfig          `json:"budget"`
	Permissions  permissions.Policy    `json:"permissions,omitempty"`
	// Retrieve is optional; omitted → topk=32, hops=1.
	Retrieve *RetrieveConfig `json:"retrieve,omitempty"`
}

// RetrieveConfig is an additive Phase B request field (clients may omit).
type RetrieveConfig struct {
	TopK int `json:"topk,omitempty"`
	Hops int `json:"hops,omitempty"`
}

// StateRef selects org state by handle or inline entities.
type StateRef struct {
	Handle   string         `json:"handle,omitempty"`
	Entities []state.Entity `json:"entities,omitempty"`
}

// BudgetConfig configures token budget enforcement.
type BudgetConfig struct {
	TokenBudget int `json:"token_budget"`
}
