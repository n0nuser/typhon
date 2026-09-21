package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/n0nuser/typhon/internal/api"
	"github.com/n0nuser/typhon/internal/search"
)

func TestMoveIsAlwaysLegalAndOnTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ruleset string
		timeout int
		req     func() api.GameRequest
		banned  []string
	}{
		{
			name: "boxed into a corner", timeout: 500,
			req: func() api.GameRequest {
				return request("standard", 500, 1,
					wire("me", 90, coords(0, 0, 0, 1, 0, 2)),
					wire("them", 90, coords(5, 5, 5, 6, 5, 7)))
			},
			banned: []string{"down", "left", "up"},
		},
		{
			name: "a very small budget still answers", timeout: 40,
			req: func() api.GameRequest {
				return request("standard", 40, 12,
					wire("me", 70, coords(5, 5, 5, 4, 5, 3)),
					wire("them", 70, coords(7, 7, 7, 6, 7, 5)))
			},
		},
		{
			name: "wrapped is played as wrapped", ruleset: "wrapped", timeout: 500,
			req: func() api.GameRequest {
				// Hemmed in on a bounded board, but on a torus Up wraps to the
				// bottom edge and is perfectly safe.
				return request("wrapped", 500, 30,
					wire("me", 90, coords(5, 10, 5, 9, 5, 8)),
					wire("them", 90, coords(1, 1, 1, 2, 1, 3)))
			},
			banned: []string{"down"},
		},
		{
			name: "an unsupported ruleset still returns a legal move", ruleset: "snail_mode", timeout: 500,
			req: func() api.GameRequest {
				return request("snail_mode", 500, 5,
					wire("me", 90, coords(0, 0, 0, 1, 0, 2)),
					wire("them", 90, coords(5, 5, 5, 6, 5, 7)))
			},
			banned: []string{"down", "left", "up"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := New(api.InfoResponse{APIVersion: "1"}, search.DefaultConfig(),
				time.Minute, slog.New(slog.DiscardHandler))

			start := time.Now()
			move := postMove(t, h, tc.req())
			took := time.Since(start)

			for _, bad := range tc.banned {
				if move.Move == bad {
					t.Errorf("played %q, which is fatal", move.Move)
				}
			}
			if move.Move == "" {
				t.Error("returned no move at all")
			}
			budget := time.Duration(tc.timeout) * time.Millisecond
			if took > budget {
				t.Errorf("took %v against a %v timeout", took, budget)
			}
		})
	}
}

// The engine sends a final /move for a snake that has already been eliminated.
// It must not panic, and it must still answer.
func TestAMoveForASnakeThatIsGoneStillAnswers(t *testing.T) {
	t.Parallel()

	h := New(api.InfoResponse{APIVersion: "1"}, search.DefaultConfig(),
		time.Minute, slog.New(slog.DiscardHandler))

	req := request("standard", 500, 40, wire("them", 90, coords(5, 5, 5, 4, 5, 3)))
	req.You = wire("me", 0, coords(1, 1, 1, 2, 1, 3))

	if got := postMove(t, h, req); got.Move == "" {
		t.Error("returned no move")
	}
}

// One server can back several snakes in one match, so the state must be keyed
// by game *and* snake. Keying by game alone would have them share a search
// table and a latency estimate.
func TestGamesAreKeyedByGameAndSnake(t *testing.T) {
	t.Parallel()

	h := New(api.InfoResponse{APIVersion: "1"}, search.DefaultConfig(),
		time.Minute, slog.New(slog.DiscardHandler))

	base := request("standard", 500, 1,
		wire("me", 90, coords(1, 1, 1, 2, 1, 3)),
		wire("other", 90, coords(9, 9, 9, 8, 9, 7)))

	postStart(t, h, base)
	second := base
	second.You = base.Board.Snakes[1]
	postStart(t, h, second)

	if got := h.store.count(); got != 2 {
		t.Errorf("store holds %d games, want 2 - one per snake", got)
	}

	postEnd(t, h, base)
	if got := h.store.count(); got != 1 {
		t.Errorf("store holds %d games after one /end, want 1", got)
	}
	postEnd(t, h, second)
	if got := h.store.count(); got != 0 {
		t.Errorf("store holds %d games after both /end, want 0", got)
	}
}

// A match the engine abandons never sends /end, so the store has to drop it on
// its own or it is a leak with a timer on it.
func TestAbandonedGamesAreSweptOut(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := newStore(time.Minute)
	s.now = func() time.Time { return now }

	s.get("abandoned/snake")
	if got := s.count(); got != 1 {
		t.Fatalf("store holds %d, want 1", got)
	}

	now = now.Add(2 * time.Minute)
	s.get("fresh/snake")
	if got := s.count(); got != 1 {
		t.Errorf("store holds %d after the sweep, want only the fresh game", got)
	}
}

// The engine's reported latency includes our own think time. Treating it as
// pure network cost charges our compute twice and ratchets the budget down
// turn after turn until the bot is searching one ply.
func TestTheBudgetDoesNotChargeOurOwnThinkTimeTwice(t *testing.T) {
	t.Parallel()

	const (
		timeout   = 500 * time.Millisecond
		roundTrip = 420 * time.Millisecond
		thought   = 400 * time.Millisecond
	)

	g := &game{}
	for range 20 {
		g.noteTurn(roundTrip, thought, thought)
	}

	// The part we cannot see is 20ms, so the budget should stay near
	// 500 - 20 - margin. A naive implementation subtracts the whole 420.
	// This turn spent nothing past its budget, so the overshoot term is zero
	// and must not eat into it either.
	got := g.budget(timeout)
	if got < 400*time.Millisecond {
		t.Errorf("budget collapsed to %v after twenty turns; the estimate is eating our own compute", got)
	}
	if got > timeout {
		t.Errorf("budget %v exceeds the timeout %v", got, timeout)
	}
}

func TestUnsupportedRulesetIsAnnounced(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if _, supported := variantFor("snail_mode", log); supported {
		t.Error("snail_mode reported as supported")
	}
	if !bytes.Contains(buf.Bytes(), []byte("snail_mode")) {
		t.Errorf("nothing was logged about the unsupported ruleset; got %q", buf.String())
	}
}

func TestInfoWebhook(t *testing.T) {
	t.Parallel()

	want := api.InfoResponse{
		APIVersion: "1", Author: "n0nuser",
		Color: "#8A0303", Head: "lantern-fish", Tail: "cosmic-horror", Version: "0.1.0",
	}
	h := New(want, search.DefaultConfig(), time.Minute, slog.New(slog.DiscardHandler))

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	var got api.InfoResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("info = %+v, want %+v", got, want)
	}
}

func postMove(t *testing.T, h *Handler, req api.GameRequest) api.MoveResponse {
	t.Helper()
	rec := post(t, h, "/move", req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /move = %d: %s", rec.Code, rec.Body.String())
	}
	var out api.MoveResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode move: %v", err)
	}
	return out
}

func postStart(t *testing.T, h *Handler, req api.GameRequest) { post(t, h, "/start", req) }
func postEnd(t *testing.T, h *Handler, req api.GameRequest)   { post(t, h, "/end", req) }

func post(t *testing.T, h *Handler, path string, req api.GameRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	return rec
}

func request(ruleset string, timeout, turn int, snakes ...api.Battlesnake) api.GameRequest {
	return api.GameRequest{
		Game: api.Game{
			ID:      fmt.Sprintf("game-%s", ruleset),
			Ruleset: api.Ruleset{Name: ruleset},
			Timeout: timeout,
		},
		Turn: turn,
		Board: api.Board{
			Width: 11, Height: 11,
			Food:   coords(5, 5),
			Snakes: snakes,
		},
		You: snakes[0],
	}
}

func wire(id string, health int, body []api.Coord) api.Battlesnake {
	return api.Battlesnake{
		ID: id, Name: id, Health: health,
		Body: body, Head: body[0], Length: len(body), Latency: "40",
	}
}

func coords(xy ...int) []api.Coord {
	out := make([]api.Coord, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, api.Coord{X: xy[i], Y: xy[i+1]})
	}
	return out
}

// A map this module does not understand must be announced, for the same reason
// an unsupported ruleset is: several official maps place their walls as hazard
// squares, and this module reads hazards as damage rather than as obstacles.
// Playing one anyway is the right call; playing it silently is not.
func TestUnfamiliarMapIsAnnounced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mapName   string
		wantKnown bool
	}{
		{name: "the default board", mapName: "standard", wantKnown: true},
		{name: "royale", mapName: "royale", wantKnown: true},
		{name: "an absent map field", mapName: "", wantKnown: true},
		{name: "a maze", mapName: "arcade_maze", wantKnown: false},
		{name: "snail mode", mapName: "snail_mode", wantKnown: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

			if got := checkMap(tc.mapName, log); got != tc.wantKnown {
				t.Errorf("checkMap(%q) = %v, want %v", tc.mapName, got, tc.wantKnown)
			}
			warned := bytes.Contains(buf.Bytes(), []byte(tc.mapName)) && tc.mapName != ""
			if !tc.wantKnown && !warned {
				t.Errorf("nothing was logged about %q; got %q", tc.mapName, buf.String())
			}
			if tc.wantKnown && buf.Len() > 0 {
				t.Errorf("warned about a known map: %q", buf.String())
			}
		})
	}
}

// The budget has to close the loop, not just avoid double-charging.
//
// A turn costs the engine three things: the network, the search, and whatever
// happens between the search's deadline expiring and the bytes leaving the
// process - encode, write, and on a throttled instance the scheduler freezing
// us mid-encode until the next CPU period. The first two were modelled. The
// third was assumed to be free, which it is on an idle machine and is not on
// Render's 0.1-CPU free tier.
//
// This plays the loop the way the engine sees it and asserts the one thing
// that matters: the round trip lands inside the timeout.
func TestTheBudgetConvergesBelowTheTimeoutOnASlowInstance(t *testing.T) {
	t.Parallel()

	const (
		timeout = 500 * time.Millisecond
		// What the network and the engine's own handling cost.
		network = 20 * time.Millisecond
		// What encode, write and a CPU-throttle freeze cost after the search
		// has already stopped. Small on a fast box, tens of ms on 0.1 CPU.
		afterSearch = 60 * time.Millisecond
	)

	g := &game{}
	var worst time.Duration

	for turn := range 20 {
		budget := g.budget(timeout)
		thought := budget + afterSearch
		roundTrip := thought + network

		// The first turn has no history to learn from; every turn after it
		// does, and none of them may be late.
		if turn > 0 && roundTrip > timeout {
			t.Errorf("turn %d: round trip %v exceeds the %v timeout (budget %v)",
				turn, roundTrip, timeout, budget)
		}
		if roundTrip > worst {
			worst = roundTrip
		}

		g.noteTurn(roundTrip, thought, budget)
	}

	if worst > timeout {
		t.Errorf("worst round trip over the game was %v against a %v timeout", worst, timeout)
	}
}

// The cost after the search stops is bimodal on a throttled instance, and an
// average of a bimodal cost is wrong in both directions.
//
// Measured over one live game on Render: either under a millisecond or 76-85ms,
// with almost nothing between, because the scheduler's freeze either lands
// after the deadline or it does not. This alternates the two and asserts what
// the engine asserts - that no reply is late.
func TestTheBudgetSurvivesABimodalPostSearchCost(t *testing.T) {
	t.Parallel()

	const (
		timeout = 500 * time.Millisecond
		network = 40 * time.Millisecond
		frozen  = 85 * time.Millisecond
	)

	g := &game{}
	var late, total int

	for turn := range 60 {
		budget := g.budget(timeout)

		// Every third turn the scheduler freezes us after the search.
		after := time.Duration(0)
		if turn%3 == 0 {
			after = frozen
		}
		thought := budget + after
		roundTrip := thought + network

		total++
		if turn > 0 && roundTrip > timeout {
			late++
			t.Errorf("turn %d: round trip %v exceeds %v (budget %v, after-search %v)",
				turn, roundTrip, timeout, budget, after)
		}
		g.noteTurn(roundTrip, thought, budget)
	}

	if late > 0 {
		t.Errorf("%d of %d turns were late", late, total)
	}

	// And the budget must still be worth having: a bot that answers on time by
	// not thinking has solved the wrong problem.
	if got := g.budget(timeout); got < 250*time.Millisecond {
		t.Errorf("budget collapsed to %v; the estimate is too conservative to play with", got)
	}
}

// A trivial turn must not teach the estimate anything.
//
// When the game is already decided the search returns in microseconds, so
// `latency - thought` charges the whole round trip as overhead. One live game
// logged 472ms of "overhead" that way. Under a peak-hold estimate that sample
// would stick and crush the budget for the rest of the game.
func TestATrivialTurnDoesNotPoisonTheEstimate(t *testing.T) {
	t.Parallel()

	const timeout = 500 * time.Millisecond

	g := &game{}
	budget := g.budget(timeout)

	// Twenty ordinary turns to establish a sane estimate.
	for range 20 {
		g.noteTurn(budget+60*time.Millisecond+20*time.Millisecond, budget+60*time.Millisecond, budget)
		budget = g.budget(timeout)
	}
	settled := g.budget(timeout)

	// Now the game ends: the search returns immediately and the engine reports
	// the full round trip.
	g.noteTurn(472*time.Millisecond, 95*time.Microsecond, budget)

	if got := g.budget(timeout); got != settled {
		t.Errorf("a turn that never searched moved the budget from %v to %v", settled, got)
	}
}
