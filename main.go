package main

import (
	"database/sql"
	"facade/system"
	"log"

	_ "modernc.org/sqlite"
)

const (
	na = "North America"
	sa = "South America"
	as = "Asia"
	eu = "Europe"
	me = "Middle East"
	eternus = "Eternus"
	ritualist = "Ritualist"
	initiate = "Initiate"
)


func main() {
	user := system.NewUser(1, "hunter", na, eternus)
	db, err := sql.Open("sqlite", "file:matchmaking.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	err = system.CreateUserTable(db)
	if err != nil {
		log.Fatal(err)
	}
	err = system.CreateLobbyTable(db)
	if err != nil {
		log.Fatal(err)
	}
	err = system.CreateClaimTable(db)
	if err != nil {
		log.Fatal(err)
	}
	
	err = system.SaveUser(db, user)
	if err != nil {
		log.Fatal(err)
	}
}