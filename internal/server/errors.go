package server

import "errors"

// errNotOnBoard reports a request whose own snake is missing from the board,
// which the engine sends on the turn after we are eliminated.
var errNotOnBoard = errors.New("server: our snake is not on the board")
