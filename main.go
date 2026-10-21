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

	for _, create := range []func(*sql.DB) error{
		system.CreateServerTable,
		system.CreateAllocationTable,
		system.CreateSessionTable,
		system.CreateProgressTable,
	} {
		if err := create(db); err != nil {
			log.Fatal(err)
		}
	}

	// Seed servers only when the database has no existing servers.
	// This avoids inserting duplicate IDs after restarting the program.
	servers, err := system.GetAvailableServers(db)
	if err != nil {
		log.Fatal(err)
	}

	if len(servers) == 0 {
		for i := 0; i < 3; i++ {
			for r, region := range []string{na, sa, as, eu, me} {
				if err := system.SaveServer(
					db,
					system.NewServer(i*5+r+1, region),
				); err != nil {
					log.Fatal(err)
				}
			}
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
		fmt.Println("7. Demo without Step C")
		fmt.Println("8. Demo Strategy patterns")
		fmt.Println("0. Exit")
		fmt.Print("Your choice: ")

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Println("Input error:", err)
			}
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
				switch {
				case errors.Is(err, system.ErrLatencyTooHigh):
					fmt.Println("B failed; A was compensated:", err)
				case errors.Is(err, system.ErrCheatDetected):
					fmt.Println("C failed; B then A were compensated:", err)
				default:
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
				for _, state := range states {
					fmt.Printf("%+v\n", state)
				}
			}

		case "6":
			id := readInt(scanner, "Claim ID: ")
			if err := admin.Compensate(id); err != nil {
				fmt.Println("Compensation:", err)
			} else {
				fmt.Println("Compensation saved.")
			}

		case "7":
			req, err := readRequest(scanner, "1")
			if err != nil {
				fmt.Println("Input error:", err)
				continue
			}

			result, err := workflow.CreateMatchSkipC(req)
			if err != nil {
				fmt.Println("Match without Step C failed:", err)
			} else {
				fmt.Println("Step C was skipped. Result:", result)
			}

		case "8":
			demoStrategies(db)

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
	if !scanner.Scan() {
		return system.MatchRequest{}, errors.New("input ended")
	}
	username := strings.TrimSpace(scanner.Text())

	if id <= 0 || username == "" {
		return system.MatchRequest{}, system.ErrInvalidInput
	}

	fmt.Println("Region: 1 North America, 2 South America, 3 Asia, 4 Europe, 5 Middle East")
	regionChoice := readInt(scanner, "Region: ")

	regions := map[int]string{
		1: na,
		2: sa,
		3: as,
		4: eu,
		5: me,
	}
	region, ok := regions[regionChoice]
	if !ok {
		return system.MatchRequest{}, system.ErrInvalidInput
	}

	fmt.Println("Rank: 1 Initiate, 2 Ritualist, 3 Eternus")
	rankChoice := readInt(scanner, "Rank: ")

	ranks := map[int]string{
		1: initiate,
		2: ritualist,
		3: eternus,
	}
	rank, ok := ranks[rankChoice]
	if !ok {
		return system.MatchRequest{}, system.ErrInvalidInput
	}

	latency := 35
	switch mode {
	case "2":
		latency = 999
	case "3":
	default:
		latency = readInt(scanner, "Latency in ms (0-150): ")
	}

	return system.MatchRequest{
		User:                system.NewUser(id, username, region, rank),
		LobbyID:             id,
		LatencyMS:           latency,
		ForceCheatDetection: mode == "3",
	}, nil
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

func demoStrategies(db *sql.DB) {
	fmt.Println("\nStrategy family 1: Server selection")

	servers, err := system.GetAvailableServers(db)
	if err != nil {
		fmt.Println("Load available servers:", err)
		return
	}
	if len(servers) == 0 {
		fmt.Println("No available servers.")
		return
	}

	req := system.SelectionRequest{
		Region:  as,
		Servers: servers,
	}

	serverContext := system.NewServerSelector(&system.NearestRegion{})
	server, err := serverContext.Select(req)
	printSelectedServer("Nearest region", server, err)

	roundRobin := &system.RoundRobinStrategy{}
	serverContext.SetStrategy(roundRobin)

	server, err = serverContext.Select(req)
	printSelectedServer("Round robin, first call", server, err)

	server, err = serverContext.Select(req)
	printSelectedServer("Round robin, second call", server, err)

	serverStrategySelector := system.NewServerStrategySelector()
	serverStrategySelector.Register(
		system.ModeNearestRegion,
		&system.NearestRegion{},
	)
	serverStrategySelector.Register(
		system.ModeLowestLoad,
		system.NewLowestLoad(db),
	)
	serverStrategySelector.Register(
		system.ModeRoundRobin,
		roundRobin,
	)

	strategy, err := serverStrategySelector.Choose(system.ModeLowestLoad)
	if err != nil {
		fmt.Println("Choose server strategy:", err)
		return
	}

	serverContext.SetStrategy(strategy)
	server, err = serverContext.Select(req)
	printSelectedServer("Automatically selected: lowest load", server, err)

	fmt.Println("\nStrategy family 2: Latency validation")

	latencyContext := system.NewLatencyValidator(
		&system.StrictLatencyStrategy{},
	)
	latencyRequest := system.LatencyRequest{
		LatencyMS:         120,
		JitterMS:          40,
		PacketLossPercent: 0.5,
	}

	fmt.Println("Latency being tested: 120 ms")

	err = latencyContext.Validate(latencyRequest)
	fmt.Println("Strict strategy:", validationResult(err))

	latencyContext.SetStrategy(&system.StandardLatencyStrategy{})
	err = latencyContext.Validate(latencyRequest)
	fmt.Println("Standard strategy:", validationResult(err))

	latencyContext.SetStrategy(&system.RelaxedLatencyStrategy{})
	err = latencyContext.Validate(latencyRequest)
	fmt.Println("Relaxed strategy:", validationResult(err))

	latencyStrategySelector := system.NewLatencyStrategySelector()
	strategy2, err := latencyStrategySelector.Choose(system.TierStandard)
	if err != nil {
		fmt.Println("Choose latency strategy:", err)
		return
	}

	latencyContext.SetStrategy(strategy2)
	err = latencyContext.Validate(latencyRequest)
	fmt.Println("Automatically selected: standard tier:", validationResult(err))
}

func printSelectedServer(label string, server *system.Server, err error) {
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}

	fmt.Printf("%s: server ID %d\n", label, server.ID)
}

func validationResult(err error) string {
	if err != nil {
		return err.Error()
	}
	return "accepted"
}
