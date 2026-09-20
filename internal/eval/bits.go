package eval

import "math/bits"

// trailingZeros is math/bits.TrailingZeros64, named locally so the bit-walking
// loops read as board scanning rather than as arithmetic.
func trailingZeros(v uint64) int { return bits.TrailingZeros64(v) }
