package system

import "errors"

var (
	ErrInvalidRank       = errors.New("invalid rank")
	ErrNoLobby           = errors.New("no lobbies found")
	ErrLatencyTooHigh    = errors.New("latency too high")
	ErrCheatDetected     = errors.New("cheat detected")
	ErrIllegalTransition = errors.New("illegal status transition")
	ErrNoServer          = errors.New("no available server")
	ErrInvalidInput      = errors.New("invalid matchmaking input")
)

type User struct {
	ID       int
	Username string
	Location string
	Rank     string
}

type Lobby struct {
	LobbyID int
}

type Claim struct {
	ID       int
	PlayerID int
	LobbyID  int
	status   string
}

func NewUser(id int, username, location, rank string) *User {
	return &User{ID: id, Username: username, Location: location, Rank: rank}
}
func NewLobby(lobbyID int) *Lobby { return &Lobby{LobbyID: lobbyID} }
func NewClaim(playerID int) *Claim {
	return &Claim{ID: playerID, PlayerID: playerID, status: "WAITING"}
}
func (c *Claim) Status() string { return c.status }

func (c *Claim) ReleaseParty() error {
	if c.status != "HELD" {
		return ErrIllegalTransition
	}
	c.status = "RELEASED"
	return nil
}
