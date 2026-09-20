package board

// FillScratch holds the working sets a flood fill needs.
//
// It is caller-owned and reused so that a fill inside the search allocates
// nothing. A GC pause inside a turn budget is a missed deadline, which makes
// this less of a micro-optimisation than it looks.
type FillScratch struct {
	free    Bitset
	reached Bitset
	next    Bitset
}

// NewFillScratch returns scratch space sized for the board.
func (t Topology) NewFillScratch() *FillScratch {
	return &FillScratch{
		free:    t.NewBitset(),
		reached: t.NewBitset(),
		next:    t.NewBitset(),
	}
}

// Reach returns the squares reachable from start without entering blocked.
//
// The result aliases scratch, so a caller that needs to keep it across another
// fill must Clone it. It is empty when start is itself blocked.
func (t Topology) Reach(blocked Bitset, start Point, scratch *FillScratch) Bitset {
	scratch.reached.Reset()
	if blocked.Has(start) {
		return scratch.reached
	}

	// Dilating into the free squares and stopping when nothing new appears.
	// The frontier cannot grow more times than there are squares.
	scratch.free.CopyFrom(blocked)
	for y := range scratch.free {
		scratch.free[y] = ^scratch.free[y] & t.rowMask()
	}

	scratch.reached.Set(start)
	for {
		t.Dilate(scratch.next, scratch.reached)
		scratch.next.And(scratch.free)
		if scratch.next.Equal(scratch.reached) {
			return scratch.reached
		}
		scratch.reached.CopyFrom(scratch.next)
	}
}

// ReachableCount returns how many squares are reachable from start, including
// start itself.
//
// This is the primary survival signal: a move into a pocket smaller than the
// snake is a self-collision that has not happened yet.
func (t Topology) ReachableCount(blocked Bitset, start Point, scratch *FillScratch) int {
	return t.Reach(blocked, start, scratch).Count()
}

// Reaches reports whether target is reachable from start.
//
// Reaching our own tail matters more than raw space. A snake that can still
// path to its tail survives indefinitely by following it, because the tail
// keeps vacating squares ahead of the head. A large open area with no route
// back to the tail is how a snake walks into a trap several turns before the
// trap closes, and counting squares alone cannot see that.
func (t Topology) Reaches(blocked Bitset, start, target Point, scratch *FillScratch) bool {
	return t.Reach(blocked, start, scratch).Has(target)
}
