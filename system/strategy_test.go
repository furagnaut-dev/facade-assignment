package system

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func newStrategyDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	for _, create := range []func(*sql.DB) error{
		CreateServerTable,
		CreateAllocationTable,
	} {
		if err := create(db); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

func TestAllServerStrategies(t *testing.T) {
	db := newStrategyDB(t)

	servers := []*Server{
		NewServer(1, "Asia"),
		NewServer(2, "Asia"),
		NewServer(3, "Europe"),
	}

	for _, server := range servers {
		if err := SaveServer(db, server); err != nil {
			t.Fatal(err)
		}
	}

	if err := SaveAllocation(db, &Allocation{
		ClaimID:  101,
		ServerID: 1,
		status:   "ALLOCATED",
	}); err != nil {
		t.Fatal(err)
	}

	req := SelectionRequest{
		Region:  "Asia",
		Servers: servers,
	}

	tests := []struct {
		name     string
		strategy ServerSelectionStrategy
		wantID   int
	}{
		{"nearest region", &NearestRegion{}, 1},
		{"lowest load", NewLowestLoad(db), 2},
		{"round robin first call", &RoundRobinStrategy{}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.strategy.Select(req)
			if err != nil {
				t.Fatalf("Select() error: %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("selected ID = %d, want %d", got.ID, tt.wantID)
			}
		})
	}
}

func TestServerStrategiesWithNoServers(t *testing.T) {
	db := newStrategyDB(t)

	tests := []struct {
		name     string
		strategy ServerSelectionStrategy
	}{
		{"nearest region", &NearestRegion{}},
		{"lowest load", NewLowestLoad(db)},
		{"round robin", &RoundRobinStrategy{}},
	}

	req := SelectionRequest{Region: "Asia"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.strategy.Select(req)
			if !errors.Is(err, ErrNoServer) {
				t.Fatalf("error = %v, want ErrNoServer", err)
			}
		})
	}
}

func TestServerSelectorSwitchesStrategy(t *testing.T) {
	servers := []*Server{
		NewServer(1, "Asia"),
		NewServer(2, "Asia"),
	}

	context := NewServerSelector(&NearestRegion{})
	req := SelectionRequest{Region: "Asia", Servers: servers}

	first, err := context.Select(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != 1 {
		t.Fatalf("nearest selected %d, want 1", first.ID)
	}

	roundRobin := &RoundRobinStrategy{}
	context.SetStrategy(roundRobin)

	first, err = context.Select(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := context.Select(req)
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != 1 || second.ID != 2 {
		t.Fatalf("round robin selected %d then %d, want 1 then 2",
			first.ID, second.ID)
	}
}

func TestServerStrategySelector(t *testing.T) {
	db := newStrategyDB(t)
	selector := NewServerStrategySelector()

	selector.Register(ModeNearestRegion, &NearestRegion{})
	selector.Register(ModeLowestLoad, NewLowestLoad(db))
	selector.Register(ModeRoundRobin, &RoundRobinStrategy{})

	tests := []struct {
		mode SelectionMode
		want string
	}{
		{ModeNearestRegion, "*system.NearestRegion"},
		{ModeLowestLoad, "*system.LowestLoadStrategy"},
		{ModeRoundRobin, "*system.RoundRobinStrategy"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got, err := selector.Choose(tt.mode)
			if err != nil {
				t.Fatal(err)
			}

			switch tt.want {
			case "*system.NearestRegion":
				if _, ok := got.(*NearestRegion); !ok {
					t.Fatalf("got %T", got)
				}
			case "*system.LowestLoadStrategy":
				if _, ok := got.(*LowestLoadStrategy); !ok {
					t.Fatalf("got %T", got)
				}
			case "*system.RoundRobinStrategy":
				if _, ok := got.(*RoundRobinStrategy); !ok {
					t.Fatalf("got %T", got)
				}
			}
		})
	}

	_, err := selector.Choose(SelectionMode("unknown"))
	if !errors.Is(err, ErrUnknownStrategy) {
		t.Fatalf("error = %v, want ErrUnknownStrategy", err)
	}
}
func TestAllLatencyStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy LatencyStrategy
		limit    int
	}{
		{"strict", &StrictLatencyStrategy{}, 100},
		{"standard", &StandardLatencyStrategy{}, 150},
		{"relaxed", &RelaxedLatencyStrategy{}, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate(LatencyRequest{
				LatencyMS: tt.limit,
			})
			if err != nil {
				t.Fatalf("limit %d rejected: %v", tt.limit, err)
			}

			err = tt.strategy.Validate(LatencyRequest{
				LatencyMS: tt.limit + 1,
			})
			if !errors.Is(err, ErrLatencyTooHigh) {
				t.Fatalf("error = %v, want ErrLatencyTooHigh", err)
			}

			err = tt.strategy.Validate(LatencyRequest{
				LatencyMS: -1,
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLatencyStrategySelector(t *testing.T) {
	selector := NewLatencyStrategySelector()

	tests := []struct {
		tier ServiceTier
		want string
	}{
		{TierStrict, "*system.StrictLatencyStrategy"},
		{TierStandard, "*system.StandardLatencyStrategy"},
		{TierRelaxed, "*system.RelaxedLatencyStrategy"},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got, err := selector.Choose(tt.tier)
			if err != nil {
				t.Fatal(err)
			}

			switch tt.want {
			case "*system.StrictLatencyStrategy":
				if _, ok := got.(*StrictLatencyStrategy); !ok {
					t.Fatalf("got %T", got)
				}
			case "*system.StandardLatencyStrategy":
				if _, ok := got.(*StandardLatencyStrategy); !ok {
					t.Fatalf("got %T", got)
				}
			case "*system.RelaxedLatencyStrategy":
				if _, ok := got.(*RelaxedLatencyStrategy); !ok {
					t.Fatalf("got %T", got)
				}
			}
		})
	}

	_, err := selector.Choose(ServiceTier("unknown"))
	if !errors.Is(err, ErrUnknownStrategy) {
		t.Fatalf("error = %v, want ErrUnknownStrategy", err)
	}
}

func TestLatencyValidatorSwitchesStrategy(t *testing.T) {
	validator := NewLatencyValidator(&StrictLatencyStrategy{})
	req := LatencyRequest{LatencyMS: 120}

	if err := validator.Validate(req); !errors.Is(err, ErrLatencyTooHigh) {
		t.Fatalf("strict error = %v, want ErrLatencyTooHigh", err)
	}

	validator.SetStrategy(&StandardLatencyStrategy{})
	if err := validator.Validate(req); err != nil {
		t.Fatalf("standard unexpectedly rejected: %v", err)
	}

	validator.SetStrategy(&RelaxedLatencyStrategy{})
	if err := validator.Validate(req); err != nil {
		t.Fatalf("relaxed unexpectedly rejected: %v", err)
	}
}


type fakeAllocationLoadReader struct {
	counts map[int]int
	err    error
}

func (f *fakeAllocationLoadReader) ActiveAllocations(serverID int) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	count, ok := f.counts[serverID]
	if !ok {
		return 0, errors.New("server load not configured in fake")
	}
	return count, nil
}

func TestLowestLoadUsesFakeReader(t *testing.T) {
	reader := &fakeAllocationLoadReader{
		counts: map[int]int{
			1: 4,
			2: 1,
			3: 0,
		},
	}

	strategy := NewLowestLoadWithReader(reader)
	req := SelectionRequest{
		Region: "Asia",
		Servers: []*Server{
			NewServer(1, "Asia"),
			NewServer(2, "Asia"),
			NewServer(3, "Europe"),
		},
	}

	got, err := strategy.Select(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Fatalf("selected server %d, want 2", got.ID)
	}
}

func TestLowestLoadPropagatesReaderError(t *testing.T) {
	readerErr := errors.New("load source unavailable")
	strategy := NewLowestLoadWithReader(&fakeAllocationLoadReader{
		err: readerErr,
	})

	_, err := strategy.Select(SelectionRequest{
		Region: "Asia",
		Servers: []*Server{
			NewServer(1, "Asia"),
		},
	})
	if !errors.Is(err, readerErr) {
		t.Fatalf("error = %v, want %v", err, readerErr)
	}
}

func TestLatencyStrategiesApplyDifferentPolicies(t *testing.T) {
	req := LatencyRequest{
		LatencyMS:         120,
		JitterMS:          40,
		PacketLossPercent: 0.5,
	}

	tests := []struct {
		name    string
		strategy LatencyStrategy
		wantErr error
	}{
		{"strict rejects latency", &StrictLatencyStrategy{}, ErrLatencyTooHigh},
		{"standard rejects jitter", &StandardLatencyStrategy{}, ErrJitterTooHigh},
		{"relaxed accepts request", &RelaxedLatencyStrategy{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate(req)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
