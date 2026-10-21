package system

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	na = "North America"
	sa = "South America"
	as = "Asia"
	eu = "Europe"
	me = "Middle East"
)

type Server struct {
	ID     int
	Region string
	status string
}

func NewServer(id int, region string) *Server {
	return &Server{ID: id, Region: region, status: "AVAILABLE"}
}
func (s *Server) Status() string { return s.status }

type Allocation struct {
	ClaimID  int
	ServerID int
	status   string
}

func (a *Allocation) Status() string { return a.status }
func (a *Allocation) Release() error {
	if a.status != "ALLOCATED" {
		return ErrIllegalTransition
	}
	a.status = "RELEASED"
	return nil
}

type LatencyChecker struct{ MaxLatencyMS int }

func (l LatencyChecker) Check(ms int) error {
	if ms < 0 {
		return ErrInvalidInput
	}
	if ms > l.MaxLatencyMS {
		return fmt.Errorf("%w: %dms", ErrLatencyTooHigh, ms)
	}
	return nil
}

type ServerAllocator struct {
	db      *sql.DB
	latency LatencyChecker
	strategySelector *ServerStrategySelector
}

func NewServerAllocator(db *sql.DB) *ServerAllocator {
	strategySelector := NewServerStrategySelector()

	strategySelector.Register(ModeNearestRegion, &NearestRegion{})
	strategySelector.Register(ModeLowestLoad, NewLowestLoad(db))
	strategySelector.Register(ModeRoundRobin, &RoundRobinStrategy{})
	return &ServerAllocator{db: db, latency: LatencyChecker{MaxLatencyMS: 150}, strategySelector: strategySelector}
}

func (s *ServerAllocator) selectServer(region string, servers []*Server, mode SelectionMode) (*Server, error) {
    req := SelectionRequest{
        Region:  region,
        Servers: servers,
        Mode:    mode,
    }

    strategy, err := s.strategySelector.Choose(mode)
    if err != nil {
        return nil, err
    }

    selector := NewServerSelector(strategy)
    return selector.Select(req)
}

func (s *ServerAllocator) Process(claimID int, region string, latencyMS int) error {
	if err := s.latency.Check(latencyMS); err != nil {
		return err
	}
	availableServers, err := GetAvailableServers(s.db)
	if err != nil {
		return err
	}
	server, err := s.selectServer(
    	region,
    	availableServers,
    	ModeLowestLoad,
	)
	if err != nil {
    	return err
	}
	server.status = "BUSY"
	if err := UpdateServer(s.db, server); err != nil {
		return err
	}
	a := &Allocation{ClaimID: claimID, ServerID: server.ID, status: "ALLOCATED"}
	if err := SaveAllocation(s.db, a); err != nil {
		server.status = "AVAILABLE"
		_ = UpdateServer(s.db, server)
		return err
	}
	return nil
}
func (s *ServerAllocator) Compensate(claimID int) error {
	a, err := FindAllocation(s.db, claimID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := a.Release(); err != nil {
		return err
	}
	if err := UpdateAllocation(s.db, a); err != nil {
		return err
	}
	server := &Server{ID: a.ServerID, status: "AVAILABLE"}
	return UpdateServer(s.db, server)
}

