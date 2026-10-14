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
}

func NewServerAllocator(db *sql.DB) *ServerAllocator {
	return &ServerAllocator{db: db, latency: LatencyChecker{MaxLatencyMS: 150}}
}
func (s *ServerAllocator) Process(claimID int, region string, latencyMS int) error {
	if err := s.latency.Check(latencyMS); err != nil {
		return err
	}
	server, err := FindAvailableServer(s.db, region)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoServer
		}
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
	return SaveProgress(s.db, claimID, ChainPendingC)
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
