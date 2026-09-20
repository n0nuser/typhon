package board

import "errors"

// ErrBoardSize reports a board this package cannot represent.
//
// Occupancy is one uint64 per row, so a board wider than MaxWidth has no
// representation here. It is refused at construction rather than silently
// truncated, because a truncated board plays a game that is not the one the
// engine is running.
var ErrBoardSize = errors.New("board: unrepresentable board size")
