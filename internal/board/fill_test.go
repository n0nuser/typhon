package board

import "testing"

func TestReach(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wrapped bool
		blocked []Point
		start   Point
		want    int
	}{
		{
			name:  "empty board reaches everything",
			start: Point{5, 5},
			want:  121,
		},
		{
			name:  "empty board from a corner reaches everything",
			start: Point{0, 0},
			want:  121,
		},
		{
			// A full-height wall down column 5 leaves 11x5 on each side.
			name:    "a wall splits a bounded board",
			blocked: column(5, 11),
			start:   Point{0, 0},
			want:    55,
		},
		{
			// The same wall on a torus splits nothing: the snake goes round
			// the outside. This is the case that a bounded flood fill gets
			// confidently, silently wrong.
			name:    "a wall does not split a wrapped board",
			wrapped: true,
			blocked: column(5, 11),
			start:   Point{0, 0},
			want:    110,
		},
		{
			name:    "a blocked start reaches nothing",
			blocked: []Point{{5, 5}},
			start:   Point{5, 5},
			want:    0,
		},
		{
			name:    "a sealed pocket reaches only itself",
			blocked: []Point{{1, 0}, {0, 1}},
			start:   Point{0, 0},
			want:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			topo := mustTopology(t, 11, 11, tc.wrapped)
			blocked := topo.NewBitset()
			for _, p := range tc.blocked {
				blocked.Set(p)
			}
			scratch := topo.NewFillScratch()

			if got := topo.ReachableCount(blocked, tc.start, scratch); got != tc.want {
				t.Errorf("ReachableCount = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReaches(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	blocked := topo.NewBitset()
	for _, p := range column(5, 11) {
		blocked.Set(p)
	}
	scratch := topo.NewFillScratch()

	if !topo.Reaches(blocked, Point{0, 0}, Point{4, 10}, scratch) {
		t.Error("Reaches said the near side was unreachable")
	}
	if topo.Reaches(blocked, Point{0, 0}, Point{6, 0}, scratch) {
		t.Error("Reaches crossed a solid wall")
	}
}

// The scratch space is reused across calls, so a second fill must not be
// contaminated by the first. This is the bug that would make the search's
// space estimate depend on which node it visited previously.
func TestReachIsIndependentOfScratchReuse(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	scratch := topo.NewFillScratch()

	open := topo.NewBitset()
	sealed := topo.NewBitset()
	sealed.Set(Point{1, 0})
	sealed.Set(Point{0, 1})

	first := topo.ReachableCount(open, Point{5, 5}, scratch)
	pocket := topo.ReachableCount(sealed, Point{0, 0}, scratch)
	again := topo.ReachableCount(open, Point{5, 5}, scratch)

	if first != 121 || again != 121 {
		t.Errorf("open board = %d then %d, want 121 both times", first, again)
	}
	if pocket != 1 {
		t.Errorf("sealed pocket = %d, want 1", pocket)
	}
}

func BenchmarkReach(b *testing.B) {
	topo, err := NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}
	blocked := topo.NewBitset()
	for _, p := range column(5, 9) {
		blocked.Set(p)
	}
	for _, p := range []Point{{2, 2}, {2, 3}, {2, 4}, {8, 6}, {8, 7}, {8, 8}} {
		blocked.Set(p)
	}
	scratch := topo.NewFillScratch()

	b.ReportAllocs()
	for b.Loop() {
		if n := topo.ReachableCount(blocked, Point{0, 0}, scratch); n == 0 {
			b.Fatal("reached nothing")
		}
	}
}

func column(x, height int) []Point {
	out := make([]Point, 0, height)
	for y := range height {
		out = append(out, Point{X: x, Y: y})
	}
	return out
}
