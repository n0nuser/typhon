package board

import (
	"slices"
	"testing"
)

func TestVoronoiSplitsAnEmptyBoard(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	blocked := topo.NewBitset()
	sources := []Source{
		{Head: Point{0, 5}, Length: 3},
		{Head: Point{10, 5}, Length: 3},
	}
	scratch := topo.NewVoronoiScratch(len(sources))

	own := topo.Voronoi(blocked, sources, scratch)

	left, right := own.CountFor(0), own.CountFor(1)
	if left != right {
		t.Errorf("symmetric heads split %d/%d, want an even split", left, right)
	}

	// Column 5 is equidistant from both heads, and the snakes are the same
	// length, so every cell in it is a mutual kill and belongs to nobody.
	// This is the opening position that killed the predecessor over and over.
	for y := range 11 {
		if got := own.Owner[topo.Index(Point{5, y})]; got != Contested {
			t.Errorf("cell {5,%d} owner = %d, want Contested", y, got)
		}
	}
	if left+right+11 != topo.Cells() {
		t.Errorf("owned %d + %d + 11 contested != %d cells", left, right, topo.Cells())
	}
}

func TestVoronoiGivesContestedCellsToTheLongerSnake(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	blocked := topo.NewBitset()
	sources := []Source{
		{Head: Point{0, 5}, Length: 5},
		{Head: Point{10, 5}, Length: 3},
	}
	scratch := topo.NewVoronoiScratch(len(sources))

	own := topo.Voronoi(blocked, sources, scratch)

	for y := range 11 {
		if got := own.Owner[topo.Index(Point{5, y})]; got != 0 {
			t.Errorf("cell {5,%d} owner = %d, want the longer snake (0)", y, got)
		}
	}
	if own.CountFor(0) <= own.CountFor(1) {
		t.Errorf("longer snake owns %d, shorter owns %d; want the longer to win the tie",
			own.CountFor(0), own.CountFor(1))
	}
	for i := range own.Owner {
		if own.Owner[i] == Contested {
			t.Fatalf("cell %v is Contested, but the lengths differ", topo.At(i))
		}
	}
}

// Ownership must not depend on the order the sources arrive in. It would be
// very easy for it to, and the symptom would be a search whose evaluation
// changes when a snake dies and the slice shifts.
func TestVoronoiIsIndependentOfSourceOrder(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	blocked := topo.NewBitset()

	forward := []Source{
		{Head: Point{1, 1}, Length: 4},
		{Head: Point{9, 9}, Length: 6},
		{Head: Point{1, 9}, Length: 4},
	}
	reversed := []Source{forward[2], forward[1], forward[0]}

	first := slices.Clone(topo.Voronoi(blocked, forward, topo.NewVoronoiScratch(3)).Owner)
	second := topo.Voronoi(blocked, reversed, topo.NewVoronoiScratch(3)).Owner

	remap := map[int16]int16{0: 2, 1: 1, 2: 0, Unowned: Unowned, Contested: Contested}
	for i := range first {
		if want := remap[second[i]]; first[i] != want {
			t.Fatalf("cell %v owned by %d forward and %d reversed",
				topo.At(i), first[i], second[i])
		}
	}
}

func TestVoronoiUnreachableCellsAreUnowned(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, false)
	blocked := topo.NewBitset()
	// Seal the bottom-left corner off from everything.
	blocked.Set(Point{1, 0})
	blocked.Set(Point{0, 1})

	sources := []Source{{Head: Point{10, 10}, Length: 3}}
	own := topo.Voronoi(blocked, sources, topo.NewVoronoiScratch(1))

	if got := own.Owner[topo.Index(Point{0, 0})]; got != Unowned {
		t.Errorf("sealed corner owner = %d, want Unowned", got)
	}
	if got := own.Owner[topo.Index(Point{1, 0})]; got != Unowned {
		t.Errorf("a blocked cell owner = %d, want Unowned", got)
	}
}

// Determinism is the contract the whole benchmark harness rests on, so it is
// tested as a property - run it twice, compare - rather than against a stored
// expectation, which would assert today's partition instead of the property.
func TestVoronoiIsDeterministic(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 11, true)
	blocked := topo.NewBitset()
	for _, p := range column(3, 7) {
		blocked.Set(p)
	}
	sources := []Source{
		{Head: Point{0, 0}, Length: 3},
		{Head: Point{6, 6}, Length: 3},
		{Head: Point{10, 2}, Length: 5},
	}
	scratch := topo.NewVoronoiScratch(len(sources))

	first := slices.Clone(topo.Voronoi(blocked, sources, scratch).Owner)
	for range 20 {
		if got := topo.Voronoi(blocked, sources, scratch).Owner; !slices.Equal(first, got) {
			t.Fatal("Voronoi returned a different partition for identical input")
		}
	}
}

func BenchmarkVoronoi(b *testing.B) {
	topo, err := NewTopology(11, 11, false)
	if err != nil {
		b.Fatal(err)
	}
	blocked := topo.NewBitset()
	for _, p := range []Point{
		{2, 2},
		{2, 3},
		{2, 4},
		{3, 4},
		{8, 6},
		{8, 7},
		{8, 8},
		{7, 8},
		{5, 0},
		{5, 1},
	} {
		blocked.Set(p)
	}
	sources := []Source{
		{Head: Point{1, 1}, Length: 4},
		{Head: Point{9, 9}, Length: 6},
		{Head: Point{1, 9}, Length: 4},
		{Head: Point{9, 1}, Length: 5},
	}
	scratch := topo.NewVoronoiScratch(len(sources))

	b.ReportAllocs()
	for b.Loop() {
		if own := topo.Voronoi(blocked, sources, scratch); own.CountFor(0) == 0 {
			b.Fatal("first source owns nothing")
		}
	}
}
