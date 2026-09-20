package main

import "errors"

// errNotPlaying reports a board this snake has already been eliminated from.
var errNotPlaying = errors.New("bench: snake is not on the board")
