package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"facade/system"

	_ "modernc.org/sqlite"
)

const (
	na        = "North America"
	sa        = "South America"
	as        = "Asia"
	eu        = "Europe"
	me        = "Middle East"
	eternus   = "Eternus"
	ritualist = "Ritualist"
	initiate  = "Initiate"
)

func main() {
	db, err := sql.Open("sqlite", "file:matchmaking.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	if _, err := system.NewParty(db); err != nil {
		log.Fatal(err)
	}
	for _, create := range []func(*sql.DB) error{system.CreateServerTable, system.CreateAllocationTable, system.CreateSessionTable, system.CreateProgressTable} {
		if err := create(db); err != nil {
			log.Fatal(err)
		}
	}
	for id, region := range []string{na, sa, as, eu, me} {
		if err := system.SaveServer(db, system.NewServer(id+1, region)); err != nil {
			log.Fatal(err)
		}
	}
	workflow := system.NewMatchmakingFacade(db)
	admin := system.NewAdminFacade(db)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\nMatchmaking")
		fmt.Println("1. Create match (success)")
		fmt.Println("2. Demo B failure (high latency; compensates A)")
		fmt.Println("3. Demo C failure (anti-cheat; compensates B then A)")
		fmt.Println("4. Inspect persisted match")
		fmt.Println("5. List matches")
		fmt.Println("6. Admin: compensate pending match")
		fmt.Println("0. Exit")
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "1", "2", "3":
			req, err := readRequest(scanner, choice)
			if err != nil {
				fmt.Println("Input error:", err)
				continue
			}
			result, err := workflow.CreateMatch(req)
			if result.ClaimID != 0 {
				fmt.Printf("Claim ID: %d\n", result.ClaimID)
			}
			if err != nil {
				if errors.Is(err, system.ErrLatencyTooHigh) {
					fmt.Println("B failed; A was compensated:", err)
				} else if errors.Is(err, system.ErrCheatDetected) {
					fmt.Println("C failed; B then A were compensated:", err)
				} else {
					fmt.Println("Match failed:", err)
				}
			} else {
				fmt.Println("Match completed successfully.")
			}
		case "4":
			id := readInt(scanner, "Claim ID: ")
			state, err := admin.Inspect(id)
			if err != nil {
				fmt.Println("Inspect:", err)
			} else {
				fmt.Printf("%+v\n", state)
			}
		case "5":
			states, err := workflow.List()
			if err != nil {
				fmt.Println("List:", err)
			} else if len(states) == 0 {
				fmt.Println("No matches yet.")
			} else {
				for _, s := range states {
					fmt.Printf("%+v\n", s)
				}
			}
		case "6":
			id := readInt(scanner, "Claim ID: ")
			if err := admin.Compensate(id); err != nil {
				fmt.Println("Compensation:", err)
			} else {
				fmt.Println("Compensation saved.")
			}
		case "0":
			return
		default:
			fmt.Println("Choose one of the listed options.")
		}
	}
}

func readRequest(scanner *bufio.Scanner, mode string) (system.MatchRequest, error) {
	id := readInt(scanner, "Player ID (use a new positive number): ")
	fmt.Print("Username: ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())
	fmt.Println("Region: 1 North America, 2 South America, 3 Asia, 4 Europe, 5 Middle East")
	regionChoice := readInt(scanner, "Region: ")
	regions := map[int]string{1: na, 2: sa, 3: as, 4: eu, 5: me}
	region, ok := regions[regionChoice]
	if !ok {
		return system.MatchRequest{}, system.ErrInvalidInput
	}
	fmt.Println("Rank: 1 Initiate, 2 Ritualist, 3 Eternus")
	rankChoice := readInt(scanner, "Rank: ")
	ranks := map[int]string{1: initiate, 2: ritualist, 3: eternus}
	rank, ok := ranks[rankChoice]
	if !ok {
		return system.MatchRequest{}, system.ErrInvalidInput
	}
	latency := 35
	if mode == "2" {
		latency = 999
	} else if mode != "3" {
		latency = readInt(scanner, "Latency in ms (0-150): ")
	}
	return system.MatchRequest{User: system.NewUser(id, username, region, rank), LobbyID: id, LatencyMS: latency, ForceCheatDetection: mode == "3"}, nil
}

func readInt(scanner *bufio.Scanner, prompt string) int {
	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			return 0
		}
		value, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err == nil {
			return value
		}
		fmt.Println("Enter a whole number.")
	}
}
