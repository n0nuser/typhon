package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/n0nuser/typhon/internal/api"
	"github.com/n0nuser/typhon/internal/board"
	"github.com/n0nuser/typhon/internal/search"
)

// defaultTimeout is used when a request omits the game timeout.
const defaultTimeout = 500 * time.Millisecond

// Limits are the deadline knobs, kept together because they are read from the
// environment and are the ones worth changing on a live instance without a
// rebuild.
type Limits struct {
	// TTL drops a game from memory after this long without contact, for
	// matches the engine abandons without sending /end.
	TTL time.Duration
	// Ceiling is the most of the engine's timeout the search may be given,
	// whatever the measured estimates say. Zero takes budgetCeiling.
	Ceiling float64
	// MaxDepth stops the iterative deepening at this ply. Zero is no cap.
	MaxDepth int
}

// Handler serves the Battlesnake webhooks.
type Handler struct {
	info   api.InfoResponse
	cfg    search.Config
	limits Limits
	store  *store
	log    *slog.Logger
	now    func() time.Time
}

// New builds a Handler.
func New(info api.InfoResponse, cfg search.Config, limits Limits, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		info:   info,
		cfg:    cfg,
		limits: limits,
		store:  newStore(limits.TTL, limits.Ceiling),
		log:    log,
		now:    time.Now,
	}
}

// Routes returns the mux serving the four webhooks.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.handleInfo)
	mux.HandleFunc("POST /start", h.handleStart)
	mux.HandleFunc("POST /move", h.handleMove)
	mux.HandleFunc("POST /end", h.handleEnd)
	return mux
}

func (h *Handler) handleInfo(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, h.info)
}

func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decode(w, r)
	if !ok {
		return
	}
	h.store.get(stateKey(req))

	_, supported := variantFor(req.Game.Ruleset.Name, h.log)
	knownMap := checkMap(req.Game.Map, h.log)
	h.log.Info("game started",
		"game", req.Game.ID, "snake", req.You.ID,
		"ruleset", req.Game.Ruleset.Name, "supported", supported,
		"known_map", knownMap,
		"map", req.Game.Map, "timeout_ms", req.Game.Timeout,
		"board", req.Board.Width, "snakes", len(req.Board.Snakes),
		"hazard_damage", req.Game.Ruleset.Settings.HazardDamagePerTurn)

	w.WriteHeader(http.StatusOK)
}

// handleMove is the only handler with a deadline, and the only one that is not
// allowed to miss it. A late answer is scored as no answer: the engine moves us
// up, and up is usually into something.
func (h *Handler) handleMove(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	req, ok := h.decode(w, r)
	if !ok {
		return
	}

	g := h.store.get(stateKey(req))
	g.mu.Lock()
	defer g.mu.Unlock()

	timeout := timeoutOf(req)
	budget := g.budget(timeout)

	decision, res := h.decide(req, g, start, budget)
	h.writeJSON(w, api.MoveResponse{Move: decision.String()})

	// After the write, so that `thought` covers everything the engine is
	// waiting on: the decode, the search, the encode and the write. What it
	// does not cover is the network, which is what `overhead` is for.
	thought := h.now().Sub(start)
	if latency, err := strconv.ParseInt(req.You.Latency, 10, 64); err == nil {
		g.noteTurn(time.Duration(latency)*time.Millisecond, thought, budget)
	}

	g.turns++
	g.depthSum += res.Depth
	g.nodes += res.Nodes
	if res.Depth > g.maxDepth {
		g.maxDepth = res.Depth
	}
	if res.Depth == 0 {
		g.fallbacks++
	}
	if res.Aborted {
		g.aborted++
	}
	if res.AllLosing {
		g.allLosing++
	}
	if thought > g.maxThink {
		g.maxThink = thought
	}
	if thought > timeout {
		g.overruns++
		h.log.Error("turn exceeded the engine's timeout",
			"game", req.Game.ID, "turn", req.Turn, "took", thought, "timeout", timeout)
	}

	h.log.Debug("move",
		"game", req.Game.ID, "turn", req.Turn, "move", decision,
		"depth", res.Depth, "score", res.Score, "nodes", res.Nodes,
		"aborted", res.Aborted, "all_losing", res.AllLosing,
		"health", req.You.Health, "length", req.You.Length,
		"budget", budget, "overhead", g.overhead, "overshoot", g.overshoot, "took", thought)
}

// decide returns the move to play. It is never allowed to return nothing.
func (h *Handler) decide(req api.GameRequest, g *game, start time.Time, budget time.Duration) (board.Direction, search.Result) {
	state, me, err := stateFrom(req, h.log)
	if err != nil {
		// Without a board there is nothing to reason about, and the engine
		// still expects a word. Up is as good as any: if we cannot build the
		// state we do not know which way is safe.
		if !errors.Is(err, errNotOnBoard) {
			h.log.Error("could not build the board", "game", req.Game.ID, "turn", req.Turn, "err", err)
		}
		return board.Up, search.Result{}
	}

	if g.searcher == nil || g.topo != state.Topo {
		// One searcher, and therefore one transposition table, per game. Two
		// games sharing a table would make each one's move depend on what the
		// other happened to look up.
		g.searcher = search.New(state.Topo, h.cfg)
		g.topo = state.Topo
	}

	// The deadline runs from the top of the handler, so the decode is spent out
	// of the same allowance as the search rather than on top of it.
	res := g.searcher.Search(state, me, search.Budget{
		Deadline: start.Add(budget),
		MaxDepth: h.limits.MaxDepth,
	})
	return res.Move, res
}

func (h *Handler) handleEnd(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decode(w, r)
	if !ok {
		return
	}

	if g := h.store.end(stateKey(req)); g != nil {
		g.mu.Lock()
		mean := 0
		if g.turns > 0 {
			mean = g.depthSum / g.turns
		}
		h.log.Info("game ended",
			"game", req.Game.ID, "snake", req.You.ID, "turns", req.Turn,
			"alive", stillAlive(req),
			"mean_depth", mean, "max_depth", g.maxDepth, "nodes", g.nodes,
			"fallbacks", g.fallbacks, "aborted_depths", g.aborted,
			"all_losing_turns", g.allLosing,
			"engine_overhead", g.overhead, "post_search", g.overshoot, "max_think", g.maxThink,
			"timeout_overruns", g.overruns)
		g.mu.Unlock()
	}

	w.WriteHeader(http.StatusOK)
}

// stateKey identifies one snake in one game.
//
// The game id alone is not enough: this server can back several snakes in the
// same match, which is how a one-against-three test is arranged. Keying on the
// game would have them share a search table and a latency estimate.
func stateKey(req api.GameRequest) string {
	return req.Game.ID + "/" + req.You.ID
}

func timeoutOf(req api.GameRequest) time.Duration {
	if req.Game.Timeout > 0 {
		return time.Duration(req.Game.Timeout) * time.Millisecond
	}
	return defaultTimeout
}

func stillAlive(req api.GameRequest) bool {
	for i := range req.Board.Snakes {
		if req.Board.Snakes[i].ID == req.You.ID {
			return true
		}
	}
	return false
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request) (api.GameRequest, bool) {
	var req api.GameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("bad request body", "path", r.URL.Path, "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return api.GameRequest{}, false
	}
	return req, true
}

func (h *Handler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already written, so this can only be logged.
		h.log.Error("write response", "err", err)
	}
}
