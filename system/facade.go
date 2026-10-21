package system

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	ChainPendingB    = "PENDING_B"
	ChainPendingC    = "PENDING_C"
	ChainCompleted   = "COMPLETED"
	ChainCompensated = "COMPENSATED"
	eternus          = "Eternus"
	ritualist        = "Ritualist"
	initiate         = "Initiate"
)

type MatchRequest struct {
	User                *User
	LobbyID             int
	LatencyMS           int
	ForceCheatDetection bool
}
type MatchResult struct{ ClaimID int }
type MatchState struct {
	ClaimID          int
	ChainStatus      string
	ClaimStatus      string
	AllocationStatus string
	SessionStatus    string
}

type MatchmakingFacade struct {
	db       *sql.DB
	party    *Party
	servers  *ServerAllocator
	sessions *SessionLogger
}

func NewMatchmakingFacade(db *sql.DB) *MatchmakingFacade {
	party, _ := NewParty(db)
	_ = CreateServerTable(db)
	_ = CreateAllocationTable(db)
	_ = CreateSessionTable(db)
	_ = CreateProgressTable(db)
	return &MatchmakingFacade{db: db, party: party, servers: NewServerAllocator(db), sessions: NewSessionLogger(db)}
}
func (f *MatchmakingFacade) BeginMatch(req MatchRequest) (int, error) {
	claim, err := f.party.PartyFormation(req.User, req.LobbyID)
	if err != nil {
		return 0, err
	}
	if err := SaveProgress(f.db, claim.ID, ChainPendingB); err != nil {
		_ = f.party.Compensate(claim.ID)
		return 0, err
	}
	return claim.ID, nil
}
func (f *MatchmakingFacade) CreateMatch(req MatchRequest) (MatchResult, error) {
	id, err := f.BeginMatch(req)
	if err != nil {
		return MatchResult{}, err
	}
	result := MatchResult{ClaimID: id}
	return result, f.Resume(id, req)
}

func (f *MatchmakingFacade) Resume(id int, req MatchRequest) error {
	status, err := FindProgress(f.db, id)
	if err != nil {
		return err
	}
	if status == ChainPendingB {
		if err := f.servers.Process(id, req.User.Location, req.LatencyMS); err != nil {
			compErr := f.party.Compensate(id)
			_ = SaveProgress(f.db, id, ChainCompensated)
			if compErr != nil {
				return fmt.Errorf("B failed: %v; A compensation failed: %w", err, compErr)
			}
			return err
		}
		if err := SaveProgress(f.db, id, ChainPendingC); err != nil {
			return err
		}
		status = ChainPendingC
	}
	if status == ChainPendingC {
		if err := f.sessions.Finalize(id, req.ForceCheatDetection); err != nil {
			bErr := f.servers.Compensate(id)
			aErr := f.party.Compensate(id)
			_ = SaveProgress(f.db, id, ChainCompensated)
			if bErr != nil || aErr != nil {
				return fmt.Errorf("C failed: %v; B compensation: %v; A compensation: %v", err, bErr, aErr)
			}
			return err
		}
		if err := SaveProgress(f.db, id, ChainCompleted); err != nil {
			return err
		}
	}
	if status == ChainCompleted {
		return nil
	}
	if status == ChainCompensated {
		return errors.New("match has already been compensated")
	}
	return nil
}
func (f *MatchmakingFacade) Inspect(id int) (MatchState, error) {
	state := MatchState{ClaimID: id}
	var err error
	state.ChainStatus, err = FindProgress(f.db, id)
	if err != nil {
		return state, err
	}
	claim, err := FindClaim(f.db, id)
	if err != nil {
		return state, err
	}
	state.ClaimStatus = claim.Status()
	if a, e := FindAllocation(f.db, id); e == nil {
		state.AllocationStatus = a.Status()
	}
	if s, e := FindSession(f.db, id); e == nil {
		state.SessionStatus = s.Status()
	}
	return state, nil
}
func (f *MatchmakingFacade) List() ([]MatchState, error) {
	rows, err := f.db.Query(`SELECT ClaimID FROM match_progress ORDER BY ClaimID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchState
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		s, err := f.Inspect(id)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type AdminFacade struct{ workflow *MatchmakingFacade }

func NewAdminFacade(db *sql.DB) *AdminFacade              { return &AdminFacade{workflow: NewMatchmakingFacade(db)} }
func (a *AdminFacade) Inspect(id int) (MatchState, error) { return a.workflow.Inspect(id) }
func (a *AdminFacade) Compensate(id int) error {
	state, err := a.workflow.Inspect(id)
	if err != nil {
		return err
	}
	if state.ChainStatus == ChainCompleted || state.ChainStatus == ChainCompensated {
		return ErrIllegalTransition
	}
	if state.AllocationStatus == "ALLOCATED" {
		if err := a.workflow.servers.Compensate(id); err != nil {
			return err
		}
	}
	if state.ClaimStatus == "HELD" {
		if err := a.workflow.party.Compensate(id); err != nil {
			return err
		}
	}
	return SaveProgress(a.workflow.db, id, ChainCompensated)
}

func (a *AdminFacade) ListByChainStatus(status string) ([]MatchState, error) {
	all, err := a.workflow.List()
	if err != nil {
		return nil, err
	}
	var out []MatchState
	for _, s := range all {
		if s.ChainStatus == status {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *MatchmakingFacade) CreateMatchSkipC(req MatchRequest) (MatchResult, error) {
	id, _ := f.BeginMatch(req)
	result := MatchResult{ClaimID: id}
	return result, SaveProgress(f.db, id, ChainCompleted)
}
