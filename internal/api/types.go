// Package api holds the Battlesnake HTTP wire types.
//
// The types mirror the official schema documented at
// https://docs.battlesnake.com/api/example-move. The board origin (0,0) is the
// bottom-left corner and y increases upward, so "up" is y+1.
//
// These are transport structs and nothing else: no logic, no defaults, no
// validation. Turning a request into the board state the search runs on is the
// job of the boundary that receives it, not of the wire format.
package api

// Coord is a single board square.
type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// RulesetSettings carries the tunable parameters of a ruleset.
//
// HazardDamagePerTurn is the field to read for royale rather than assuming a
// flat penalty. Note that the wire spells it out in full while the rules
// library's own parameter key is "damagePerTurn"; they are not the same name.
type RulesetSettings struct {
	FoodSpawnChance     int `json:"foodSpawnChance"`
	MinimumFood         int `json:"minimumFood"`
	HazardDamagePerTurn int `json:"hazardDamagePerTurn"`
}

// Ruleset identifies which rules the game engine is applying.
type Ruleset struct {
	Name     string          `json:"name"`
	Version  string          `json:"version"`
	Settings RulesetSettings `json:"settings"`
}

// Game describes the match a request belongs to.
//
// Timeout is the per-move budget in milliseconds and it includes the round trip
// between the engine and this server. Always derive deadlines from this field
// rather than assuming the usual 500.
type Game struct {
	ID      string  `json:"id"`
	Ruleset Ruleset `json:"ruleset"`
	Map     string  `json:"map"`
	Source  string  `json:"source"`
	Timeout int     `json:"timeout"`
}

// Customizations are cosmetic and carry no gameplay meaning.
type Customizations struct {
	Color string `json:"color"`
	Head  string `json:"head"`
	Tail  string `json:"tail"`
}

// Battlesnake is one snake's state for the current turn.
//
// Body is ordered head-first and may contain duplicate coordinates while a
// snake is stacked: at the start of a game, and on the turn after it eats, when
// the engine duplicates the tail. Code that releases a tail square by looking
// at the last element frees a square that is still occupied.
//
// Latency is the round trip the engine measured for our previous reply, as a
// string of milliseconds. It includes our own think time, so it is not the
// network cost on its own.
type Battlesnake struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Health         int            `json:"health"`
	Body           []Coord        `json:"body"`
	Latency        string         `json:"latency"`
	Head           Coord          `json:"head"`
	Length         int            `json:"length"`
	Shout          string         `json:"shout"`
	Customizations Customizations `json:"customizations"`
}

// Board is the full grid state for the current turn.
type Board struct {
	Height  int           `json:"height"`
	Width   int           `json:"width"`
	Food    []Coord       `json:"food"`
	Hazards []Coord       `json:"hazards"`
	Snakes  []Battlesnake `json:"snakes"`
}

// GameRequest is the body sent to /start, /move and /end.
type GameRequest struct {
	Game  Game        `json:"game"`
	Turn  int         `json:"turn"`
	Board Board       `json:"board"`
	You   Battlesnake `json:"you"`
}

// InfoResponse is the body returned by GET /.
type InfoResponse struct {
	APIVersion string `json:"apiversion"`
	Author     string `json:"author,omitempty"`
	Color      string `json:"color,omitempty"`
	Head       string `json:"head,omitempty"`
	Tail       string `json:"tail,omitempty"`
	Version    string `json:"version,omitempty"`
}

// MoveResponse is the body returned by POST /move.
type MoveResponse struct {
	Move  string `json:"move"`
	Shout string `json:"shout,omitempty"`
}
