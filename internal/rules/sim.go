package rules

import "github.com/n0nuser/typhon/internal/board"

// Undo is everything needed to put a state back the way Apply found it.
//
// It is returned by value and kept on a stack by the search, so it holds no
// slices of its own: a per-node allocation here is a GC pause inside a turn
// budget. Food is the one variable-sized part, and it lives on a stack the
// state owns, which this indexes into.
type Undo struct {
	noop      bool
	turn      int
	snakes    [MaxSnakes]snakeUndo
	foodStart int
	foodCount int
}

type snakeUndo struct {
	moved bool
	// popped is the segment that fell off the tail when the snake moved.
	popped uint16
	// grew counts this turn's growths. It reaches two only in constrictor,
	// where a snake can eat the map's starting food and then grow again by the
	// rule, in the same turn.
	grew uint8
	// clobbered holds the ring slots growth overwrote, so they can be restored.
	clobbered [2]uint16
	health    int
	cause     Cause
}

// Apply advances the state by one turn and returns the undo that reverses it.
//
// The stage order is the engine's own, read from standard.go rather than from
// the documentation prose:
//
//	GameOver -> Movement -> Starvation -> HazardDamage -> Feed -> Elimination
//
// with constrictor adding food removal and unconditional growth on the end, and
// wrapped replacing movement with its wrapping form. The order is not
// incidental. Movement always pops the tail and Feed re-duplicates it, which is
// the entire mechanism behind "a tail frees up as the snake moves, except when
// it just ate" and behind the duplicate coordinates a body carries while it is
// stacked. Hazard damage landing before Feed is why a snake that takes hazard
// damage and reaches food on the same turn is still restored to full health.
//
// moves is indexed by snake; a dead snake's entry is ignored.
func (s *State) Apply(moves *[MaxSnakes]board.Direction) Undo {
	// The engine checks for game over before it applies anything, so a state
	// with one snake left never advances. Matching that keeps the differential
	// test honest at the end of every duel, and gives the search its terminal.
	if s.Over() {
		return Undo{noop: true}
	}

	u := Undo{turn: s.Turn, foodStart: len(s.foodStack)}
	for i := range s.Snakes {
		u.snakes[i] = snakeUndo{health: s.Snakes[i].Health, cause: s.Snakes[i].Cause}
	}

	// outOfBounds records the snakes whose move left a bounded board. Their
	// bodies stay where they are rather than take an unrepresentable head: the
	// engine moves them and eliminates them in the same turn, and an eliminated
	// snake blocks nobody, so the board ends up identical either way.
	var outOfBounds [MaxSnakes]bool

	s.move(moves, &u, &outOfBounds)
	s.starve()
	s.damageHazards(&outOfBounds)
	s.feed(&u, &outOfBounds)
	s.eliminate(&outOfBounds)

	if s.Variant == Constrictor {
		s.constrict(&u)
	}

	u.foodCount = len(s.foodStack) - u.foodStart
	s.Turn++
	return u
}

// move appends a new head and pops the tail for every living snake.
func (s *State) move(moves *[MaxSnakes]board.Direction, u *Undo, outOfBounds *[MaxSnakes]bool) {
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}

		next, ok := s.Topo.Step(s.Topo.At(int(sn.Head())), moves[i])
		if !ok {
			outOfBounds[i] = true
			continue
		}

		tail := sn.Tail()
		s.vacate(tail)

		sn.head--
		if sn.head < 0 {
			sn.head = len(sn.ring) - 1
		}
		sn.ring[sn.head] = uint16(s.Topo.Index(next))
		s.occupy(sn.ring[sn.head])

		u.snakes[i].moved = true
		u.snakes[i].popped = tail
	}
}

// starve takes the one health every living snake loses per turn.
func (s *State) starve() {
	for i := range s.Snakes {
		if s.Snakes[i].Alive() {
			s.Snakes[i].Health--
		}
	}
}

// damageHazards drains the health of every snake whose head ends the turn on a
// hazard square.
//
// Food on the square cancels the damage outright - not reduces it, cancels it.
// That is in the engine and not in the prose documentation.
//
// The engine loops its hazard list per snake, so a square listed twice deals
// damage twice. This models hazards as a set and deals it once, which is
// identical for every map these rulesets use; NewState rejects a duplicate
// rather than diverging quietly.
func (s *State) damageHazards(outOfBounds *[MaxSnakes]bool) {
	if s.HazardDamage == 0 {
		return
	}
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() || outOfBounds[i] {
			continue
		}
		head := s.Topo.At(int(sn.Head()))
		if !s.Hazards.Has(head) || s.Food.Has(head) {
			continue
		}
		sn.Health -= s.HazardDamage
		if sn.Health < 0 {
			sn.Health = 0
		}
		if sn.Health > MaxHealth {
			sn.Health = MaxHealth
		}
		if sn.Health <= 0 {
			s.kill(i, Hazard)
		}
	}
}

// feed restores a snake that ended its move on food, and stacks its tail.
//
// Every snake on the square is fed, not just one, and the food is consumed
// once. The snakes then usually kill each other in the head-to-head, but that
// is the elimination stage's business, not this one's.
func (s *State) feed(u *Undo, outOfBounds *[MaxSnakes]bool) {
	var eaten [MaxSnakes]uint16
	n := 0

	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() || outOfBounds[i] {
			continue
		}
		head := sn.Head()
		if !s.Food.Has(s.Topo.At(int(head))) {
			continue
		}
		s.grow(i, u)
		sn.Health = MaxHealth

		seen := false
		for k := range n {
			if eaten[k] == head {
				seen = true
				break
			}
		}
		if !seen {
			eaten[n] = head
			n++
		}
	}

	for k := range n {
		s.Food.Clear(s.Topo.At(int(eaten[k])))
		s.foodStack = append(s.foodStack, eaten[k])
	}
}

// grow stacks a duplicate of the snake's current tail, which is how the engine
// makes growth land on the turn after the meal.
func (s *State) grow(i int, u *Undo) {
	sn := &s.Snakes[i]
	tail := sn.Tail()
	slot := sn.slot(sn.length)
	u.snakes[i].clobbered[u.snakes[i].grew] = sn.ring[slot]
	u.snakes[i].grew++
	sn.ring[slot] = tail
	sn.length++
	s.occupy(tail)
}

// eliminate removes the snakes that died this turn.
//
// Two passes, because the engine uses two and the difference is visible on the
// board. A snake that starves or leaves the board is removed first and blocks
// nobody, so a rival moving into the square it just vacated survives. A snake
// that dies to a collision is removed only after every collision has been
// collected, so a rival moving into its body dies with it. A single pass with
// immediate removal gets both of those wrong.
func (s *State) eliminate(outOfBounds *[MaxSnakes]bool) {
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}
		switch {
		case sn.Health <= 0:
			s.kill(i, OutOfHealth)
		case outOfBounds[i]:
			s.kill(i, OutOfBounds)
		}
	}

	var doomed [MaxSnakes]Cause
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		if !sn.Alive() {
			continue
		}
		head := sn.Head()

		if bodyHit(sn, head) {
			doomed[i] = SelfCollision
			continue
		}

		hit := false
		for j := range s.Snakes {
			other := &s.Snakes[j]
			if i == j || !other.Alive() {
				continue
			}
			if bodyHit(other, head) {
				doomed[i] = BodyCollision
				hit = true
				break
			}
		}
		if hit {
			continue
		}

		for j := range s.Snakes {
			other := &s.Snakes[j]
			if i == j || !other.Alive() {
				continue
			}
			// Equal lengths lose to each other, so both snakes are collected
			// and both die. A tie is exactly as fatal as a loss, which is why
			// openings full of equal-length snakes produce mutual kills.
			if other.Head() == head && sn.length <= other.length {
				doomed[i] = HeadToHead
				break
			}
		}
	}

	for i := range s.Snakes {
		if doomed[i] != Alive && s.Snakes[i].Alive() {
			s.kill(i, doomed[i])
		}
	}
}

// bodyHit reports whether cell lands on a snake's body below the head.
//
// The head is skipped because the engine handles head-on-head only through the
// head-to-head rule, never as a body collision - which is what lets the longer
// snake survive one. It also means a length-one snake can never collide with
// itself.
func bodyHit(sn *Snake, cell uint16) bool {
	for j := 1; j < sn.length; j++ {
		if sn.Cell(j) == cell {
			return true
		}
	}
	return false
}

// constrict applies the ruleset that gives the mode its name: no food, full
// health, and a segment added every turn until the board fills.
//
// The engine applies this to eliminated snakes too, setting their health and
// growing them after they are off the board, which is why a differential test
// can only compare the living. It also reads the second-to-last segment
// directly, so it would panic on a length-one snake; this checks instead,
// because a search can reach positions a real game never does.
func (s *State) constrict(u *Undo) {
	if !s.Food.IsEmpty() {
		for y := range s.Food {
			row := s.Food[y]
			for row != 0 {
				x := trailingZeros(row)
				s.foodStack = append(s.foodStack, uint16(y*s.Topo.Width+x))
				row &= row - 1
			}
		}
		s.Food.Reset()
	}
	for i := range s.Snakes {
		sn := &s.Snakes[i]
		sn.Health = MaxHealth
		if !sn.Alive() || sn.length < 2 {
			continue
		}
		if sn.Cell(sn.length-1) != sn.Cell(sn.length-2) {
			s.grow(i, u)
		}
	}
}

// kill takes a snake off the board and records why.
func (s *State) kill(i int, cause Cause) {
	s.removeBody(i)
	s.Snakes[i].Cause = cause
}

// Unapply restores the state Apply changed, exactly.
func (s *State) Unapply(u Undo) {
	if u.noop {
		return
	}

	for i := range s.Snakes {
		sn := &s.Snakes[i]
		su := u.snakes[i]

		// A snake that died this turn goes back onto the board before its body
		// is rewound, so the same cells come back that were taken away.
		if !sn.Alive() && su.cause == Alive {
			s.addBody(i)
		}
		sn.Cause = su.cause
		sn.Health = su.health

		for g := int(su.grew) - 1; g >= 0; g-- {
			sn.length--
			slot := sn.slot(sn.length)
			s.vacate(sn.ring[slot])
			sn.ring[slot] = su.clobbered[g]
		}

		if su.moved {
			s.vacate(sn.ring[sn.head])
			sn.head++
			if sn.head >= len(sn.ring) {
				sn.head = 0
			}
			s.occupy(su.popped)
		}
	}

	for k := u.foodStart + u.foodCount - 1; k >= u.foodStart; k-- {
		s.Food.Set(s.Topo.At(int(s.foodStack[k])))
	}
	s.foodStack = s.foodStack[:u.foodStart]

	s.Turn = u.turn
}
