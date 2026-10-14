package system

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := NewParty(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateServerTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateSessionTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateProgressTable(db); err != nil {
		t.Fatal(err)
	}
	if err := SaveServer(db, NewServer(1, na)); err != nil {
		t.Fatal(err)
	}
	return db
}

func validRequest(id int) MatchRequest {
	return MatchRequest{User: NewUser(id, "hunter", na, eternus), LobbyID: id, LatencyMS: 30}
}

func TestMatchCompletesAllThreeSteps(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	result, err := f.CreateMatch(validRequest(1))
	if err != nil {
		t.Fatalf("CreateMatch: %v", err)
	}
	state, err := f.Inspect(result.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ChainStatus != ChainCompleted {
		t.Fatalf("status = %q, want %q", state.ChainStatus, ChainCompleted)
	}
}

func TestBFailureCompensatesOnlyA(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	req := validRequest(2)
	req.LatencyMS = 999
	result, err := f.CreateMatch(req)
	if !errors.Is(err, ErrLatencyTooHigh) {
		t.Fatalf("error = %v, want ErrLatencyTooHigh", err)
	}
	state, err := f.Inspect(result.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ChainStatus != ChainCompensated || state.ClaimStatus != "RELEASED" {
		t.Fatalf("persisted state = %+v, want compensated chain and released claim", state)
	}
}

func TestCFailureCompensatesBThenA(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	req := validRequest(3)
	req.ForceCheatDetection = true
	result, err := f.CreateMatch(req)
	if !errors.Is(err, ErrCheatDetected) {
		t.Fatalf("error = %v, want ErrCheatDetected", err)
	}
	state, err := f.Inspect(result.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ChainStatus != ChainCompensated || state.ClaimStatus != "RELEASED" || state.AllocationStatus != "RELEASED" {
		t.Fatalf("persisted state = %+v, want A and B compensated", state)
	}
}

func TestResumeAfterDatabaseRestartUsesPersistedProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	open := func() *sql.DB {
		db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	if _, err := NewParty(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateServerTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateSessionTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateProgressTable(db); err != nil {
		t.Fatal(err)
	}
	if err := SaveServer(db, NewServer(1, na)); err != nil {
		t.Fatal(err)
	}
	f := NewMatchmakingFacade(db)
	id, err := f.BeginMatch(validRequest(4))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = open()
	defer db.Close()
	f = NewMatchmakingFacade(db)
	if err := f.Resume(id, validRequest(4)); err != nil {
		t.Fatalf("Resume after restart: %v", err)
	}
	state, err := f.Inspect(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.ChainStatus != ChainCompleted {
		t.Fatalf("status = %q, want COMPLETED", state.ChainStatus)
	}
}

func TestInvalidRankFailsBeforePersistingClaim(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	req := validRequest(5)
	req.User.Rank = "Unknown"
	if _, err := f.CreateMatch(req); !errors.Is(err, ErrInvalidRank) {
		t.Fatalf("error = %v, want ErrInvalidRank", err)
	}
}

func TestClaimRejectsIllegalTransition(t *testing.T) {
	claim := NewClaim(10)
	if err := claim.LobbyFormation(1); err != nil {
		t.Fatal(err)
	}
	if err := claim.LobbyFormation(2); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("error = %v, want ErrIllegalTransition", err)
	}
}

func TestAdminFacadeCanCompensatePendingMatch(t *testing.T) {
	db := testDB(t)
	workflow := NewMatchmakingFacade(db)
	id, err := workflow.BeginMatch(validRequest(6))
	if err != nil {
		t.Fatal(err)
	}
	admin := NewAdminFacade(db)
	if err := admin.Compensate(id); err != nil {
		t.Fatalf("admin compensation: %v", err)
	}
	state, err := admin.Inspect(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.ChainStatus != ChainCompensated || state.ClaimStatus != "RELEASED" {
		t.Fatalf("state = %+v", state)
	}
}

func TestListShowsPersistedMatches(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	if _, err := f.CreateMatch(validRequest(7)); err != nil {
		t.Fatal(err)
	}
	states, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states[0].ClaimID != 7 {
		t.Fatalf("states = %+v, want one match with ID 7", states)
	}
}

func TestSessionRejectsSecondTransition(t *testing.T) {
	s := NewSession(8)
	if err := s.Confirm(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reject(); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("error = %v, want illegal transition", err)
	}
}

func TestAllocationMustBeAllocatedBeforeRelease(t *testing.T) {
	a := &Allocation{ClaimID: 9, ServerID: 1, status: "NEW"}
	if err := a.Release(); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("error = %v, want illegal transition", err)
	}
}

func TestLatencyCheckerRejectsNegativeLatency(t *testing.T) {
	if err := (LatencyChecker{MaxLatencyMS: 150}).Check(-1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

func TestPartyCanBeUsedWithoutFacade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "standalone.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	party, err := NewParty(db)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := party.PartyFormation(NewUser(10, "solo", na, initiate), 10)
	if err != nil {
		t.Fatalf("standalone PartyFormation: %v", err)
	}
	if claim.Status() != "HELD" {
		t.Fatalf("claim status = %q, want HELD", claim.Status())
	}
}

func TestSubsystemStorageGatewaysPersistAndReload(t *testing.T) {
	db := testDB(t)
	user := NewUser(20, "storage-test", na, initiate)
	if err := SaveUser(db, user); err != nil {
		t.Fatal(err)
	}
	if err := SaveLobby(db, NewLobby(20)); err != nil {
		t.Fatal(err)
	}
	claim := NewClaim(20)
	if err := claim.LobbyFormation(20); err != nil {
		t.Fatal(err)
	}
	if err := SaveClaim(db, claim); err != nil {
		t.Fatal(err)
	}
	loaded, err := FindClaim(db, 20)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status() != "HELD" {
		t.Fatalf("loaded claim status = %q", loaded.Status())
	}
	if err := loaded.ReleaseParty(); err != nil {
		t.Fatal(err)
	}
	if err := UpdateClaim(db, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = FindClaim(db, 20)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status() != "RELEASED" {
		t.Fatalf("reloaded claim status = %q", loaded.Status())
	}
}

func TestBReportsNoServerAndFacadeCompensatesA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-server.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := NewParty(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateServerTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateAllocationTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateSessionTable(db); err != nil {
		t.Fatal(err)
	}
	if err := CreateProgressTable(db); err != nil {
		t.Fatal(err)
	}
	f := NewMatchmakingFacade(db)
	result, err := f.CreateMatch(validRequest(21))
	if !errors.Is(err, ErrNoServer) {
		t.Fatalf("error = %v, want ErrNoServer", err)
	}
	state, err := f.Inspect(result.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ClaimStatus != "RELEASED" || state.ChainStatus != ChainCompensated {
		t.Fatalf("state = %+v", state)
	}
}

func TestAdminCannotCompensateCompletedMatch(t *testing.T) {
	db := testDB(t)
	f := NewMatchmakingFacade(db)
	result, err := f.CreateMatch(validRequest(22))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAdminFacade(db).Compensate(result.ClaimID); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("error = %v, want illegal transition", err)
	}
}
