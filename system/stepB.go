package system

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
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
	db               *sql.DB
	latency          LatencyChecker
	strategySelector *ServerStrategySelector
	mu               sync.Mutex
	selector         *ServerSelector
	current string
}

func NewServerAllocator(db *sql.DB) *ServerAllocator {
	return NewServerAllocatorWithReader(db, &sqlAllocationLoadReader{db: db})
}

func NewServerAllocatorWithReader(db *sql.DB, reader AllocationLoadReader) *ServerAllocator {
	strategySelector := NewServerStrategySelector()
	strategySelector.Register(ModeNearestRegion, &NearestRegion{})
	strategySelector.Register(ModeLowestLoad, NewLowestLoadWithReader(reader))
	strategySelector.Register(ModeRoundRobin, &RoundRobinStrategy{})

	initial, _ := strategySelector.Choose(ModeLowestLoad)
	return &ServerAllocator{
		db:               db,
		latency:          LatencyChecker{MaxLatencyMS: 150},
		strategySelector: strategySelector,
		selector:         NewServerSelector(initial),
		current: string(ModeLowestLoad),
	}
}

func (s *ServerAllocator) SetMode(mode SelectionMode) error {
	strategy, err := s.strategySelector.Choose(mode)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selector.SetStrategy(strategy)
	s.current = string(mode)
	return nil
}

func (s *ServerAllocator) SetBySituation(c SelectionContext) error {
	strategy, err := s.strategySelector.ChooseFor(c)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selector.SetStrategy(strategy)
	s.current = fmt.Sprintf("auto -> %T", strategy)
	return nil
}

func (s *ServerAllocator) selectServer(region string, servers []*Server) (*Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selector.Select(SelectionRequest{Region: region, Servers: servers})
}

func (s *ServerAllocator) Current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *ServerAllocator) Process(claimID int, region string, latencyMS int) error {
	if err := s.latency.Check(latencyMS); err != nil {
		return err
	}
	availableServers, err := GetAvailableServers(s.db)
	if err != nil {
		return err
	}
	server, err := s.selectServer(region, availableServers)
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

type InMemoryLoadReader struct{ Loads map[int]int }

func (r InMemoryLoadReader) ActiveAllocations(id int) (int, error) { return r.Loads[id], nil }

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
