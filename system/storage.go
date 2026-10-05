package system

import (
	"database/sql"
)

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