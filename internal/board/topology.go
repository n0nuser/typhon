// Package board holds the geometry and occupancy primitives the search runs on.
//
// Everything here is a pure function over its arguments: no I/O, no logging, no
// globals, no clock. That is what makes it table-testable against the official
// rules and benchmarkable against a turn deadline.
//
// The board origin (0,0) is the bottom-left corner and y increases upward, so
// Up is y+1.
package board

// MaxWidth is the widest board this package represents.
//
// The occupancy bitset holds one uint64 per row, so a row must fit in a word.
// The largest official Battlesnake board is 25 wide, which makes 64 headroom
// rather than a limit anyone reaches. A wider board is rejected at the boundary
// instead of being played wrong.
const MaxWidth = 64

// Point is a square on the board.
//
// It is deliberately separate from the wire format's coordinate type so that
// this package does not depend on the transport.
type Point struct {
	X, Y int
}

// Direction is one of the four moves a snake may make.
type Direction uint8

// The four legal moves, in the order ties break.
//
// The order is load-bearing rather than arbitrary. Two moves the evaluation
// cannot separate are separated by this order, and a bot that instead picks
// between them at random wanders - which fills in its own escape routes. The
// predecessor measured a fixed preference beating a coin 18-2 on exactly these
// decisions.
const (
	Up Direction = iota
	Down
	Left
	Right
)

// Directions lists every legal move in the order ties break.
var Directions = [4]Direction{Up, Down, Left, Right}

// String returns the wire word the Battlesnake API expects.
func (d Direction) String() string {
	switch d {
	case Up:
		return "up"
	case Down:
		return "down"
	case Left:
		return "left"
	case Right:
		return "right"
	default:
		return "up"
	}
}

// Topology is the shape of the board and the only place its geometry lives.
//
// Every neighbour calculation and every distance calculation in this repository
// goes through this type. An x+1 written anywhere else is a bug: it is exactly
// how the wrapped ruleset ends up subtly wrong while every standard test stays
// green, because on a torus the edges are not edges and the metric is not
// Manhattan.
type Topology struct {
	Width   int
	Height  int
	Wrapped bool
}

// NewTopology returns the topology for a board, or an error if this package
// cannot represent it.
func NewTopology(width, height int, wrapped bool) (Topology, error) {
	if width <= 0 || height <= 0 {
		return Topology{}, ErrBoardSize
	}
	if width > MaxWidth {
		return Topology{}, ErrBoardSize
	}
	return Topology{Width: width, Height: height, Wrapped: wrapped}, nil
}

// Cells returns the number of squares on the board.
func (t Topology) Cells() int { return t.Width * t.Height }

// Index returns the flat index of a point that is already in bounds.
func (t Topology) Index(p Point) int { return p.Y*t.Width + p.X }

// At returns the point at a flat index.
func (t Topology) At(index int) Point {
	return Point{X: index % t.Width, Y: index / t.Width}
}

// InBounds reports whether p lies inside the board rectangle.
//
// This is about the raw coordinate, not about reachability: on a wrapped board
// a point outside the rectangle is still playable, it just has to be normalised
// first, which is what Step does.
func (t Topology) InBounds(p Point) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < t.Width && p.Y < t.Height
}

// Step returns the square reached by moving one cell in d.
//
// On a bounded board the second result is false when the move leaves the board,
// which is fatal. On a wrapped board the move comes back on the opposite edge
// and the second result is always true.
func (t Topology) Step(p Point, d Direction) (Point, bool) {
	n := p
	switch d {
	case Up:
		n.Y++
	case Down:
		n.Y--
	case Left:
		n.X--
	case Right:
		n.X++
	}

	if !t.Wrapped {
		return n, t.InBounds(n)
	}

	n.X = wrap(n.X, t.Width)
	n.Y = wrap(n.Y, t.Height)
	return n, true
}

// Distance returns the number of steps between two squares, ignoring bodies.
//
// It is Manhattan on a bounded board and toroidal on a wrapped one, where going
// off one edge is a shortcut to the other. Food selection and head-to-head
// contest detection both rest on this, which is why there is one of it.
func (t Topology) Distance(a, b Point) int {
	dx := abs(a.X - b.X)
	dy := abs(a.Y - b.Y)
	if t.Wrapped {
		if w := t.Width - dx; w < dx {
			dx = w
		}
		if h := t.Height - dy; h < dy {
			dy = h
		}
	}
	return dx + dy
}

// wrap normalises a coordinate that is at most one step outside [0, size).
func wrap(v, size int) int {
	switch {
	case v < 0:
		return v + size
	case v >= size:
		return v - size
	default:
		return v
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
