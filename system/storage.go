package system

import (
	"database/sql"
)

//For step a

func CreateClaimTable(db *sql.DB) error {
	_, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS claims 
		(ID INTEGER PRIMARY KEY, 
		PlayerID INTEGER NOT NULL REFERENCES users(ID), 
		Status TEXT NOT NULL, 
		LobbyID INTEGER REFERENCES lobbies(LobbyID))`)
	return err
}

func CreateLobbyTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS lobbies
	(LobbyID INTEGER PRIMARY KEY)
	`)
	return err
}

func CreateUserTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users
	(
	ID INTEGER PRIMARY KEY,
	Username TEXT NOT NULL,
	Location TEXT NOT NULL,
	RANK TEXT NOT NULL
	)`)
	return err
}

func SaveUser(db *sql.DB, user *User) error {
	_, err := db.Exec(`
		INSERT INTO users (ID, Username, Location, Rank) VALUES
		(?, ?, ?, ?)
	`, user.ID, user.Username, user.Location, user.Rank)
	return err
}

func SaveLobby(db *sql.DB, lobby *Lobby) error {
	_, err := db.Exec(
		`INSERT INTO lobbies (LobbyID) VALUES (?)`, lobby.LobbyID)
	return err
}

func SaveClaim(db *sql.DB, claim *Claim) error {
	_, err := db.Exec(
		`INSERT INTO claims (ID, PlayerID, Status, LobbyID) VALUES (?, ?, ?, ?)`,
		 claim.ID, claim.PlayerID, claim.status, claim.LobbyID)
	return err
}

func UpdateClaim(db *sql.DB, claim *Claim) error {
	_, err := db.Exec(`UPDATE claims SET Status=? WHERE ID=?`, claim.status, claim.ID)
	return err
}

func FindClaim(db *sql.DB, id int) (*Claim, error) {
	c := &Claim{}
	err := db.QueryRow(`SELECT ID, PlayerID, Status, LobbyID FROM claims WHERE ID=?`, id).
		Scan(&c.ID, &c.PlayerID, &c.status, &c.LobbyID)
	return c, err
}

//step a ends
//step b and c start

func CreateServerTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS servers (ServerID INTEGER PRIMARY KEY, Region TEXT NOT NULL, Status TEXT NOT NULL)`)
	return err
}
func SaveServer(db *sql.DB, s *Server) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO servers (ServerID, Region, Status) VALUES (?, ?, ?)`, s.ID, s.Region, s.status)
	return err
}
func FindAvailableServer(db *sql.DB, region string) (*Server, error) {
	s := &Server{}
	err := db.QueryRow(`SELECT ServerID, Region, Status FROM servers WHERE Status='AVAILABLE' AND Region=? ORDER BY ServerID LIMIT 1`, region).Scan(&s.ID, &s.Region, &s.status)
	return s, err
}
func UpdateServer(db *sql.DB, s *Server) error {
	_, err := db.Exec(`UPDATE servers SET Status=? WHERE ServerID=?`, s.status, s.ID)
	return err
}

func CreateAllocationTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS allocations (ClaimID INTEGER PRIMARY KEY, ServerID INTEGER NOT NULL REFERENCES servers(ServerID), Status TEXT NOT NULL)`)
	return err
}
func SaveAllocation(db *sql.DB, a *Allocation) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO allocations (ClaimID, ServerID, Status) VALUES (?, ?, ?)`, a.ClaimID, a.ServerID, a.status)
	return err
}
func FindAllocation(db *sql.DB, claimID int) (*Allocation, error) {
	a := &Allocation{}
	err := db.QueryRow(`SELECT ClaimID, ServerID, Status FROM allocations WHERE ClaimID=?`, claimID).Scan(&a.ClaimID, &a.ServerID, &a.status)
	return a, err
}
func UpdateAllocation(db *sql.DB, a *Allocation) error {
	_, err := db.Exec(`UPDATE allocations SET Status=? WHERE ClaimID=?`, a.status, a.ClaimID)
	return err
}

func CreateSessionTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (ClaimID INTEGER PRIMARY KEY, Status TEXT NOT NULL, Note TEXT NOT NULL)`)
	return err
}
func SaveSession(db *sql.DB, s *Session) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO sessions (ClaimID, Status, Note) VALUES (?, ?, ?)`, s.ClaimID, s.status, s.Note)
	return err
}
func FindSession(db *sql.DB, claimID int) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(`SELECT ClaimID, Status, Note FROM sessions WHERE ClaimID=?`, claimID).Scan(&s.ClaimID, &s.status, &s.Note)
	return s, err
}

func CreateProgressTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS match_progress (ClaimID INTEGER PRIMARY KEY, Status TEXT NOT NULL)`)
	return err
}
func SaveProgress(db *sql.DB, id int, status string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO match_progress (ClaimID, Status) VALUES (?, ?)`, id, status)
	return err
}
func FindProgress(db *sql.DB, id int) (string, error) {
	var status string
	err := db.QueryRow(`SELECT Status FROM match_progress WHERE ClaimID=?`, id).Scan(&status)
	return status, err
}
