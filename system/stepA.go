package system

import (
	"database/sql"
	"fmt"
)

var Ranks = []string{"Initiate", "Ritualist", "Eternus"}

type Rating struct{}

func (Rating) ValidateRating(rank string) error {
	for _, r := range Ranks {
		if r == rank {
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrInvalidRank, rank)
}

func (c *Claim) LobbyFormation(id int) error {
	if c.status != "WAITING" {
		return fmt.Errorf("%w: %q -> HELD", ErrIllegalTransition, c.status)
	}
	lobby := NewLobby(id)
	if lobby.LobbyID <= 0 {
		return ErrNoLobby
	}
	c.LobbyID = lobby.LobbyID
	c.status = "HELD"
	return nil
}

type Party struct {
	db     *sql.DB
	rating Rating
}

func NewParty(db *sql.DB) (*Party, error) {
	for _, create := range []func(*sql.DB) error{CreateUserTable, CreateLobbyTable, CreateClaimTable} {
		if err := create(db); err != nil {
			return nil, err
		}
	}
	return &Party{db: db}, nil
}

func (p *Party) PartyFormation(u *User, lobbyID int) (*Claim, error) {
	if u == nil || u.ID <= 0 || u.Username == "" || u.Location == "" {
		return nil, ErrInvalidInput
	}
	if err := p.rating.ValidateRating(u.Rank); err != nil {
		return nil, err
	}
	claim := NewClaim(u.ID)
	if err := claim.LobbyFormation(lobbyID); err != nil {
		return nil, err
	}
	if err := SaveUser(p.db, u); err != nil {
		return nil, err
	}
	if err := SaveLobby(p.db, NewLobby(lobbyID)); err != nil {
		return nil, err
	}
	return claim, SaveClaim(p.db, claim)
}

func (p *Party) Release(id int) error {
	claim, err := FindClaim(p.db, id)
	if err != nil {
		return err
	}
	if err := claim.ReleaseParty(); err != nil {
		return err
	}
	return UpdateClaim(p.db, claim)
}

func (p *Party) Compensate(id int) error { return p.Release(id) }
