package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aminmortezaie/contextcompiler/internal/compiler"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Server exposes the context compiler HTTP API.
type Server struct {
	Memory memory.Layer
}

// NewServer builds an API server with the given memory layer.
func NewServer(mem memory.Layer) *Server {
	return &Server{Memory: mem}
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/compile", s.handleCompile)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleCompile(w http.ResponseWriter, r *http.Request) {
	var req CompileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	entities, err := s.resolveState(r, req.State)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TaskContract.Question == "" && len(req.TaskContract.Keywords) == 0 {
		writeError(w, http.StatusBadRequest, "task_contract.question is required")
		return
	}
	result := compiler.Compile(entities, req.TaskContract, compileOptions(s, req, entities))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func compileOptions(s *Server, req CompileRequest, entities []state.Entity) compiler.Options {
	opts := compiler.Options{
		TokenBudget: req.Budget.TokenBudget,
		Permissions: req.Permissions,
	}
	if req.Retrieve != nil {
		opts.TopK = req.Retrieve.TopK
		opts.Hops = req.Retrieve.Hops
	}
	if len(req.State.Entities) > 0 {
		opts.Graph = memory.DefaultGraph(entities)
		opts.Handle = memory.InlineHandle
		return opts
	}
	if gs, ok := memory.AsGraphStore(s.Memory); ok {
		opts.Graph = gs
		opts.Handle = req.State.Handle
	}
	return opts
}

func (s *Server) resolveState(r *http.Request, ref StateRef) ([]state.Entity, error) {
	if len(ref.Entities) > 0 {
		return ref.Entities, nil
	}
	if ref.Handle == "" {
		return nil, errors.New("state.handle or state.entities is required")
	}
	if s.Memory == nil {
		return nil, errors.New("state handle requires a memory layer")
	}
	return s.Memory.Load(r.Context(), ref.Handle)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
