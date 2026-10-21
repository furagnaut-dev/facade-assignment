package system

import (
	"database/sql"
	"errors"
	"sync"
)

type SelectionMode string
var (
	ErrUnknownStrategy = errors.New("unknown strategy")
)
const (
	ModeNearestRegion SelectionMode = "nearest"
	ModeLowestLoad SelectionMode = "lowest_load"
	ModeRoundRobin SelectionMode = "round_robin"
)

type SelectionRequest struct {
	Region string
	Servers []*Server
	Mode SelectionMode
}

type ServerSelectionStrategy interface{
	Select(req SelectionRequest) (*Server, error)
}

type NearestRegion struct{}

func (n *NearestRegion) Select(req SelectionRequest) (*Server, error) {
	if len(req.Servers) == 0 {
		return nil, ErrNoServer
	}
	for _, server := range req.Servers {
		if server.Region == req.Region {
			return server, nil
		}
	}
	return nil, ErrNoServer
} 

type LowestLoadStrategy struct {
	db *sql.DB
}

func NewLowestLoad(db *sql.DB) *LowestLoadStrategy {
	return &LowestLoadStrategy{db: db}
}

func (l *LowestLoadStrategy) Select(req SelectionRequest) (*Server, error) {
	if len(req.Servers) == 0 {
		return nil, ErrNoServer
	}
	var selected *Server
	lowestCount := 0

	for _, server := range req.Servers {
		if server.Region != req.Region {
			continue
		}
		count, err := CountActiveAllocations(l.db, server.ID)
		if err != nil {
			return nil, err
		}
		if selected == nil || count < lowestCount {
			selected = server
			lowestCount = count
		} 
	}
	if selected == nil {
		return nil, ErrNoServer
	}
	return selected, nil
}

type RoundRobinStrategy struct {
	mu sync.Mutex
	nextIndex int
}

func (r *RoundRobinStrategy) Select(req SelectionRequest) (*Server, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(req.Servers) == 0 {
		return nil, ErrNoServer
	}

	selected := req.Servers[r.nextIndex%len(req.Servers)]
	r.nextIndex = (r.nextIndex + 1) % len(req.Servers)
	return selected, nil
}

type ServerSelector struct {
	strategy ServerSelectionStrategy
}

func NewServerSelector(strategy ServerSelectionStrategy) *ServerSelector {
	return &ServerSelector{strategy: strategy}
}

func (s *ServerSelector) SetStrategy(strategy ServerSelectionStrategy) {
	s.strategy = strategy
}

func (s *ServerSelector) Select(req SelectionRequest) (*Server, error) {
	if s.strategy == nil {
        return nil, ErrUnknownStrategy
    }
	return s.strategy.Select(req)
}

type ServerStrategySelector struct {
	strategies map[SelectionMode]ServerSelectionStrategy
}

func NewServerStrategySelector() *ServerStrategySelector {
	return &ServerStrategySelector{
		strategies: make(map[SelectionMode]ServerSelectionStrategy),
	}
}

func (s *ServerStrategySelector) Register(mode SelectionMode, strategy ServerSelectionStrategy) {
	s.strategies[mode] = strategy
}

func (s *ServerStrategySelector) Choose(mode SelectionMode) (ServerSelectionStrategy, error) {
	strategy, ok := s.strategies[mode]
	if !ok {
		return nil, ErrUnknownStrategy
	}
	return strategy, nil
}