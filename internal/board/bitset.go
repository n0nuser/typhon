package board

import "math/bits"

// Bitset is an occupancy map holding one uint64 per row: bit x of word y is set
// when the square (x, y) is occupied.
//
// The layout is what makes flood fill affordable inside a search. Expanding a
// region by one square in every direction is four shifts and three ORs per row,
// against a queue and a visited array per cell. On an 11x11 board a whole fill
// is a few dozen word operations.
type Bitset []uint64

// NewBitset returns an empty set sized for the board.
func (t Topology) NewBitset() Bitset { return make(Bitset, t.Height) }

// rowMask is the set of bits inside the board on any row.
//
// Every left shift must be masked with it, or a bit walks off the right edge of
// the board and reappears on the next row.
func (t Topology) rowMask() uint64 {
	if t.Width >= 64 {
		return ^uint64(0)
	}
	return 1<<uint(t.Width) - 1
}

// Set marks a square occupied.
func (b Bitset) Set(p Point) { b[p.Y] |= 1 << uint(p.X) }

// Clear marks a square free.
func (b Bitset) Clear(p Point) { b[p.Y] &^= 1 << uint(p.X) }

// Has reports whether a square is occupied.
func (b Bitset) Has(p Point) bool { return b[p.Y]&(1<<uint(p.X)) != 0 }

// Count returns how many squares are occupied.
func (b Bitset) Count() int {
	n := 0
	for _, row := range b {
		n += bits.OnesCount64(row)
	}
	return n
}

// IsEmpty reports whether no square is occupied.
func (b Bitset) IsEmpty() bool {
	for _, row := range b {
		if row != 0 {
			return false
		}
	}
	return true
}

// Equal reports whether two sets hold the same squares.
func (b Bitset) Equal(other Bitset) bool {
	if len(b) != len(other) {
		return false
	}
	for y, row := range b {
		if row != other[y] {
			return false
		}
	}
	return true
}

// Reset empties the set in place.
func (b Bitset) Reset() {
	for y := range b {
		b[y] = 0
	}
}

// CopyFrom overwrites b with src.
func (b Bitset) CopyFrom(src Bitset) { copy(b, src) }

// Clone returns an independent copy.
func (b Bitset) Clone() Bitset {
	out := make(Bitset, len(b))
	copy(out, b)
	return out
}

// Or adds every square of src to b.
func (b Bitset) Or(src Bitset) {
	for y := range b {
		b[y] |= src[y]
	}
}

// And keeps only the squares b and src share.
func (b Bitset) And(src Bitset) {
	for y := range b {
		b[y] &= src[y]
	}
}

// AndNot removes every square of src from b.
func (b Bitset) AndNot(src Bitset) {
	for y := range b {
		b[y] &^= src[y]
	}
}

// Points returns the occupied squares in row order.
//
// This allocates, so it belongs in tests and on the slow paths that render or
// report a board, never inside the search.
func (b Bitset) Points() []Point {
	out := make([]Point, 0, b.Count())
	for y, row := range b {
		for row != 0 {
			x := bits.TrailingZeros64(row)
			out = append(out, Point{X: x, Y: y})
			row &= row - 1
		}
	}
	return out
}

// Dilate writes into dst every square of src plus each of their four
// neighbours. dst and src must not be the same set.
//
// This is the primitive every reachability question in the package is built
// from: a flood fill is dilation masked by the free squares, repeated until
// nothing new appears, and a Voronoi partition is several such frontiers
// expanded in lockstep.
func (t Topology) Dilate(dst, src Bitset) {
	mask := t.rowMask()
	for y := 0; y < t.Height; y++ {
		row := src[y]
		var out uint64

		if t.Wrapped {
			// A rotation within the board's width, not within the word: the
			// leftmost square's left neighbour is the rightmost square.
			out = rotateLeft(row, t.Width, mask) | rotateRight(row, t.Width, mask)
			out |= src[wrap(y+1, t.Height)] | src[wrap(y-1, t.Height)]
		} else {
			out = (row << 1) & mask
			out |= row >> 1
			if y+1 < t.Height {
				out |= src[y+1]
			}
			if y-1 >= 0 {
				out |= src[y-1]
			}
		}

		dst[y] = (out | row) & mask
	}
}

// rotateLeft moves every bit one square right along the row, which is a rotate
// left of the low width bits.
func rotateLeft(row uint64, width int, mask uint64) uint64 {
	return ((row << 1) | (row >> uint(width-1))) & mask
}

// rotateRight moves every bit one square left along the row.
func rotateRight(row uint64, width int, mask uint64) uint64 {
	return ((row >> 1) | (row << uint(width-1))) & mask
}
