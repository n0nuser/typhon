package board

import (
	"slices"
	"strconv"
	"testing"
)

func TestBitsetSetClearHas(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	set := topo.NewBitset()

	points := []Point{{0, 0}, {10, 0}, {0, 10}, {10, 10}, {5, 5}}
	for _, p := range points {
		set.Set(p)
	}
	if got := set.Count(); got != len(points) {
		t.Fatalf("Count() = %d, want %d", got, len(points))
	}
	for _, p := range points {
		if !set.Has(p) {
			t.Errorf("Has(%v) = false after Set", p)
		}
	}
	if set.Has(Point{1, 1}) {
		t.Error("Has({1,1}) = true, never set")
	}

	set.Clear(Point{5, 5})
	if set.Has(Point{5, 5}) {
		t.Error("Has({5,5}) = true after Clear")
	}
	if got := set.Count(); got != len(points)-1 {
		t.Errorf("Count() = %d after Clear, want %d", got, len(points)-1)
	}
}

// A set bit must never leak past the edge of a row into the next one. That is
// the failure the row mask exists to prevent, and it is silent: the board keeps
// working and a square on the far side of the map becomes mysteriously blocked.
func TestBitsetDoesNotLeakAcrossRows(t *testing.T) {
	t.Parallel()

	for _, width := range []int{1, 7, 11, 25, 63, MaxWidth} {
		t.Run("width "+strconv.Itoa(width), func(t *testing.T) {
			t.Parallel()
			topo := mustTopology(t, width, 4, false)

			src := topo.NewBitset()
			src.Set(Point{X: width - 1, Y: 1})
			dst := topo.NewBitset()
			topo.Dilate(dst, src)

			for _, p := range dst.Points() {
				if p.X >= width {
					t.Fatalf("Dilate produced %v, outside a %d-wide board", p, width)
				}
			}
			if dst.Has(Point{X: 0, Y: 1}) && width > 1 {
				t.Error("Dilate wrapped on a bounded board")
			}
		})
	}
}

func TestDilate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wrapped bool
		seed    Point
		want    []Point
	}{
		{
			name: "bounded middle has four neighbours",
			seed: Point{5, 5},
			want: []Point{{5, 4}, {4, 5}, {5, 5}, {6, 5}, {5, 6}},
		},
		{
			// A corner on a bounded board keeps only the two neighbours that
			// exist, which is what makes corners dangerous.
			name: "bounded corner has two",
			seed: Point{0, 0},
			want: []Point{{0, 0}, {1, 0}, {0, 1}},
		},
		{
			name: "bounded top-right corner has two",
			seed: Point{10, 10},
			want: []Point{{9, 10}, {10, 10}, {10, 9}},
		},
		{
			// On a torus a corner is an ordinary square: nothing is an edge.
			name:    "wrapped corner has four",
			wrapped: true,
			seed:    Point{0, 0},
			want:    []Point{{0, 0}, {1, 0}, {10, 0}, {0, 1}, {0, 10}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			topo := mustTopology(t, 11, 11, tc.wrapped)
			src := topo.NewBitset()
			src.Set(tc.seed)
			dst := topo.NewBitset()
			topo.Dilate(dst, src)

			got := dst.Points()
			want := slices.Clone(tc.want)
			sortPoints(got)
			sortPoints(want)
			if !slices.Equal(got, want) {
				t.Errorf("Dilate(%v) = %v, want %v", tc.seed, got, want)
			}
		})
	}
}

func TestBitsetSetOperations(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	a := topo.NewBitset()
	a.Set(Point{1, 1})
	a.Set(Point{2, 2})
	b := topo.NewBitset()
	b.Set(Point{2, 2})
	b.Set(Point{3, 3})

	union := a.Clone()
	union.Or(b)
	if got := union.Count(); got != 3 {
		t.Errorf("Or count = %d, want 3", got)
	}

	inter := a.Clone()
	inter.And(b)
	if got := inter.Count(); got != 1 || !inter.Has(Point{2, 2}) {
		t.Errorf("And = %v, want only {2,2}", inter.Points())
	}

	diff := a.Clone()
	diff.AndNot(b)
	if got := diff.Count(); got != 1 || !diff.Has(Point{1, 1}) {
		t.Errorf("AndNot = %v, want only {1,1}", diff.Points())
	}

	if a.Equal(b) {
		t.Error("Equal reported two different sets as equal")
	}
	if !a.Equal(a.Clone()) {
		t.Error("Equal reported a clone as different")
	}

	empty := topo.NewBitset()
	if !empty.IsEmpty() {
		t.Error("a fresh set is not empty")
	}
	a.Reset()
	if !a.IsEmpty() {
		t.Error("Reset left something behind")
	}
}

func sortPoints(ps []Point) {
	slices.SortFunc(ps, func(a, b Point) int {
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.X - b.X
	})
}
