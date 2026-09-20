package board

import "math/bits"

// MaxSources is the largest number of snakes a Voronoi partition is sized for.
const MaxSources = 8

// Ownership of a cell that no source reaches, or that two equal sources reach
// at the same moment.
const (
	// Unowned marks a cell no source can reach.
	Unowned int16 = -1
	// Contested marks a cell two equally long sources reach on the same step,
	// where a head-to-head would eliminate both. It belongs to nobody.
	Contested int16 = -2
)

// Source is one snake's starting position for a Voronoi partition.
//
// Length decides contested cells: the longer snake survives a head-to-head, so
// it is the one that can actually use the square.
type Source struct {
	Head   Point
	Length int
}

// Ownership says which source controls each cell, indexed by Topology.Index.
type Ownership struct {
	Owner []int16
}

// CountFor returns how many cells the source at index i owns.
func (o Ownership) CountFor(i int) int {
	want := int16(i)
	n := 0
	for _, owner := range o.Owner {
		if owner == want {
			n++
		}
	}
	return n
}

// VoronoiScratch holds the working sets a partition needs, so that repeating
// one inside the search allocates nothing.
type VoronoiScratch struct {
	frontier []Bitset
	next     []Bitset
	win      []Bitset
	free     Bitset
	claimed  Bitset
	once     Bitset
	multi    Bitset
	avail    Bitset
	dilated  Bitset
	order    []int
	owner    []int16
}

// NewVoronoiScratch returns scratch space for up to sources snakes.
func (t Topology) NewVoronoiScratch(sources int) *VoronoiScratch {
	s := &VoronoiScratch{
		frontier: make([]Bitset, sources),
		next:     make([]Bitset, sources),
		win:      make([]Bitset, sources),
		free:     t.NewBitset(),
		claimed:  t.NewBitset(),
		once:     t.NewBitset(),
		multi:    t.NewBitset(),
		avail:    t.NewBitset(),
		dilated:  t.NewBitset(),
		order:    make([]int, sources),
		owner:    make([]int16, t.Cells()),
	}
	for i := range sources {
		s.frontier[i] = t.NewBitset()
		s.next[i] = t.NewBitset()
		s.win[i] = t.NewBitset()
	}
	return s
}

// Voronoi partitions the free squares by which source reaches them first.
//
// This is the space signal the evaluation is built on. Raw reachable-square
// counts say how much room a snake has in isolation; this says how much room it
// has *against the other snakes*, which is the quantity that actually decides
// games. Cutting an opponent off is visible here and invisible to a flood fill.
//
// Every source's frontier expands one step per round, simultaneously. A cell
// first reached by one source belongs to it. A cell first reached by several on
// the same round goes to the longest, because that is the snake that wins the
// head-to-head there; if the longest are tied, nobody gets it.
//
// A source keeps expanding through cells it lost, because losing a square does
// not change how far away the squares behind it are.
//
// The result aliases scratch and is valid until the next call with it.
func (t Topology) Voronoi(blocked Bitset, sources []Source, scratch *VoronoiScratch) Ownership {
	t.growScratch(scratch, len(sources))
	for i := range scratch.owner {
		scratch.owner[i] = Unowned
	}

	scratch.free.CopyFrom(blocked)
	for y := range scratch.free {
		scratch.free[y] = ^scratch.free[y] & t.rowMask()
	}
	scratch.claimed.Reset()

	// Heads sit on their own bodies, so they are blocked. Seed the wave from
	// them anyway: the question is which squares each snake reaches first, and
	// that is measured from where it stands. A head counts as space its own
	// snake controls, which keeps the partition summing to the whole board.
	for i := range sources {
		scratch.frontier[i].Reset()
		scratch.frontier[i].Set(sources[i].Head)
		scratch.claimed.Set(sources[i].Head)

		head := t.Index(sources[i].Head)
		if scratch.owner[head] == Unowned {
			scratch.owner[head] = int16(i)
		} else {
			// Two heads on one square is not a legal position, but a caller
			// that builds one gets a defined answer rather than a silent
			// last-writer-wins.
			scratch.owner[head] = Contested
		}
	}

	order := scratch.order[:len(sources)]
	byLengthDescending(order, sources)

	for {
		grew := false
		for i := range sources {
			t.Dilate(scratch.dilated, scratch.frontier[i])
			scratch.next[i].CopyFrom(scratch.dilated)
			scratch.next[i].And(scratch.free)
			scratch.next[i].AndNot(scratch.claimed)
			if !scratch.next[i].IsEmpty() {
				grew = true
			}
		}
		if !grew {
			return Ownership{Owner: scratch.owner}
		}

		t.assignRound(scratch, sources, order)

		for i := range sources {
			scratch.claimed.Or(scratch.next[i])
			scratch.frontier[i].CopyFrom(scratch.next[i])
		}
	}
}

// assignRound gives this round's newly reached cells to their owners, longest
// source first, so that a shorter snake never takes a square a longer one
// reached at the same moment.
func (t Topology) assignRound(scratch *VoronoiScratch, sources []Source, order []int) {
	scratch.avail.Reset()
	for i := range sources {
		scratch.avail.Or(scratch.next[i])
	}
	for i := range sources {
		scratch.win[i].Reset()
	}

	for start := 0; start < len(order); {
		// One group of equally long sources, which is where ties arise.
		end := start + 1
		length := sources[order[start]].Length
		for end < len(order) && sources[order[end]].Length == length {
			end++
		}

		scratch.once.Reset()
		scratch.multi.Reset()
		for _, i := range order[start:end] {
			for y := range scratch.multi {
				scratch.multi[y] |= scratch.once[y] & scratch.next[i][y]
				scratch.once[y] |= scratch.next[i][y]
			}
		}
		scratch.once.And(scratch.avail)
		scratch.multi.And(scratch.avail)

		for _, i := range order[start:end] {
			scratch.win[i].CopyFrom(scratch.next[i])
			scratch.win[i].And(scratch.once)
			scratch.win[i].AndNot(scratch.multi)
		}
		setOwner(t, scratch.owner, scratch.multi, Contested)

		// Everything this group touched is decided; a shorter group cannot
		// take it, even where this group only managed a mutual kill.
		scratch.avail.AndNot(scratch.once)
		start = end
	}

	for i := range sources {
		setOwner(t, scratch.owner, scratch.win[i], int16(i))
	}
}

// setOwner writes v into owner for every cell of set.
//
// It walks the words directly rather than calling Bitset.Points, which would
// allocate a slice on every round of every partition of every search node.
func setOwner(t Topology, owner []int16, set Bitset, v int16) {
	for y, row := range set {
		base := y * t.Width
		for row != 0 {
			owner[base+bits.TrailingZeros64(row)] = v
			row &= row - 1
		}
	}
}

// byLengthDescending fills order with source indices, longest first, breaking
// ties by index so the partition does not depend on input order.
//
// Insertion sort rather than sort.Slice: the slice holds at most MaxSources
// entries, and this allocates nothing and needs no comparator closure.
func byLengthDescending(order []int, sources []Source) {
	for i := range sources {
		order[i] = i
	}
	for i := 1; i < len(order); i++ {
		v := order[i]
		j := i - 1
		for j >= 0 && sources[order[j]].Length < sources[v].Length {
			order[j+1] = order[j]
			j--
		}
		order[j+1] = v
	}
}

// growScratch makes room for n sources, allocating only when a partition is
// asked for more snakes than the scratch was built for. Reusing the scratch
// across calls is what keeps the search allocation-free; this is the escape
// hatch for a caller that sized it wrong, not the normal path.
func (t Topology) growScratch(s *VoronoiScratch, n int) {
	for len(s.frontier) < n {
		s.frontier = append(s.frontier, t.NewBitset())
		s.next = append(s.next, t.NewBitset())
		s.win = append(s.win, t.NewBitset())
		s.order = append(s.order, 0)
	}
}
