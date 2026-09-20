package board

import "testing"

func TestNewTopologyRejectsUnrepresentableBoards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		wantErr       bool
	}{
		{name: "standard 11x11", width: 11, height: 11},
		{name: "largest official board", width: 25, height: 25},
		{name: "arcade maze", width: 19, height: 21},
		{name: "widest representable", width: MaxWidth, height: 4},
		{name: "one past the widest", width: MaxWidth + 1, height: 4, wantErr: true},
		{name: "zero width", width: 0, height: 11, wantErr: true},
		{name: "negative height", width: 11, height: -1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewTopology(tc.width, tc.height, false)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewTopology(%d, %d) error = %v, wantErr %v",
					tc.width, tc.height, err, tc.wantErr)
			}
		})
	}
}

func TestStep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wrapped bool
		from    Point
		dir     Direction
		want    Point
		wantOK  bool
	}{
		{name: "bounded up", from: Point{5, 5}, dir: Up, want: Point{5, 6}, wantOK: true},
		{name: "bounded down", from: Point{5, 5}, dir: Down, want: Point{5, 4}, wantOK: true},
		{name: "bounded left", from: Point{5, 5}, dir: Left, want: Point{4, 5}, wantOK: true},
		{name: "bounded right", from: Point{5, 5}, dir: Right, want: Point{6, 5}, wantOK: true},

		{name: "bounded off the top", from: Point{5, 10}, dir: Up, want: Point{5, 11}},
		{name: "bounded off the bottom", from: Point{5, 0}, dir: Down, want: Point{5, -1}},
		{name: "bounded off the left", from: Point{0, 5}, dir: Left, want: Point{-1, 5}},
		{name: "bounded off the right", from: Point{10, 5}, dir: Right, want: Point{11, 5}},

		{name: "wrapped off the top", wrapped: true, from: Point{5, 10}, dir: Up, want: Point{5, 0}, wantOK: true},
		{name: "wrapped off the bottom", wrapped: true, from: Point{5, 0}, dir: Down, want: Point{5, 10}, wantOK: true},
		{name: "wrapped off the left", wrapped: true, from: Point{0, 5}, dir: Left, want: Point{10, 5}, wantOK: true},
		{name: "wrapped off the right", wrapped: true, from: Point{10, 5}, dir: Right, want: Point{0, 5}, wantOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			topo := mustTopology(t, 11, 11, tc.wrapped)
			got, ok := topo.Step(tc.from, tc.dir)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("Step(%v, %v) = %v, %v; want %v, %v",
					tc.from, tc.dir, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wrapped bool
		a, b    Point
		want    int
	}{
		{name: "same square", a: Point{3, 3}, b: Point{3, 3}, want: 0},
		{name: "bounded adjacent", a: Point{3, 3}, b: Point{3, 4}, want: 1},
		{name: "bounded across the board", a: Point{0, 0}, b: Point{10, 0}, want: 10},
		{name: "bounded diagonal corners", a: Point{0, 0}, b: Point{10, 10}, want: 20},

		// The whole reason Distance is not Manhattan everywhere: on a torus
		// the far edge is one step away, and food and head-to-head logic both
		// get this wrong if they compute it themselves.
		{name: "wrapped across the board is one step", wrapped: true, a: Point{0, 0}, b: Point{10, 0}, want: 1},
		{name: "wrapped vertically", wrapped: true, a: Point{0, 0}, b: Point{0, 10}, want: 1},
		{name: "wrapped diagonal corners", wrapped: true, a: Point{0, 0}, b: Point{10, 10}, want: 2},
		{name: "wrapped takes the short way", wrapped: true, a: Point{2, 0}, b: Point{9, 0}, want: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			topo := mustTopology(t, 11, 11, tc.wrapped)
			if got := topo.Distance(tc.a, tc.b); got != tc.want {
				t.Errorf("Distance(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
			if got := topo.Distance(tc.b, tc.a); got != tc.want {
				t.Errorf("Distance is not symmetric: (%v, %v) = %d, want %d", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

func TestIndexRoundTrips(t *testing.T) {
	t.Parallel()

	topo := mustTopology(t, 11, 7, false)
	for i := range topo.Cells() {
		if got := topo.Index(topo.At(i)); got != i {
			t.Fatalf("Index(At(%d)) = %d", i, got)
		}
	}
}

func TestDirectionString(t *testing.T) {
	t.Parallel()

	want := []string{"up", "down", "left", "right"}
	for i, d := range Directions {
		if got := d.String(); got != want[i] {
			t.Errorf("Directions[%d].String() = %q, want %q", i, got, want[i])
		}
	}
}

func mustTopology(t *testing.T, width, height int, wrapped bool) Topology {
	t.Helper()
	topo, err := NewTopology(width, height, wrapped)
	if err != nil {
		t.Fatalf("NewTopology(%d, %d, %v): %v", width, height, wrapped, err)
	}
	return topo
}
