package rules

import "errors"

// Errors returned when a state cannot be built.
var (
	// ErrTooManySnakes reports a board with more snakes than MaxSnakes.
	ErrTooManySnakes = errors.New("rules: too many snakes")
	// ErrEmptySnake reports a snake with no body, which the engine treats as
	// an error rather than as a dead snake.
	ErrEmptySnake = errors.New("rules: zero-length snake")
	// ErrDuplicateHazard reports a hazard square listed more than once. The
	// engine deals its damage once per listing; this package models hazards as
	// a set and deals it once, so a duplicate is refused rather than played
	// with quietly different damage.
	ErrDuplicateHazard = errors.New("rules: duplicate hazard square")
)
