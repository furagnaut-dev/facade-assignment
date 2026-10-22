package system

import (
	"errors"
	"fmt"
)

type ServiceTier string

const (
	TierStrict   ServiceTier = "strict"
	TierStandard ServiceTier = "standard"
	TierRelaxed  ServiceTier = "relaxed"
)

var (
	ErrPacketLossTooHigh = errors.New("packets loss too high")
	ErrJitterTooHigh     = errors.New("jitter too high")
)

type LatencyRequest struct {
	LatencyMS         int
	JitterMS          int
	PacketLossPercent float64
}

type LatencyStrategy interface {
	Validate(req LatencyRequest) error
}

type StrictLatencyStrategy struct{}

func (s *StrictLatencyStrategy) Validate(req LatencyRequest) error {
	if err := validateBasicLatencyInput(req); err != nil {
		return err
	}

	if req.LatencyMS > 100 {
		return fmt.Errorf("%w: %dms exceeds strict limit of 100ms",
			ErrLatencyTooHigh, req.LatencyMS)
	}
	if req.JitterMS > 15 {
		return fmt.Errorf("%w: jitter %dms exceeds strict limit of 15ms",
			ErrJitterTooHigh, req.JitterMS)
	}
	if req.PacketLossPercent > 1 {
		return fmt.Errorf("%w: packet loss %.2f%% exceeds strict limit of 1%%",
			ErrPacketLossTooHigh, req.PacketLossPercent)
	}

	return nil
}

type StandardLatencyStrategy struct{}

func (s *StandardLatencyStrategy) Validate(req LatencyRequest) error {
	if err := validateBasicLatencyInput(req); err != nil {
		return err
	}

	if req.LatencyMS > 150 {
		return fmt.Errorf("%w: %dms exceeds standard limit of 150ms",
			ErrLatencyTooHigh, req.LatencyMS)
	}
	if req.JitterMS > 30 {
		return fmt.Errorf("%w: jitter %dms exceeds standard limit of 30ms",
			ErrJitterTooHigh, req.JitterMS)
	}

	return nil
}

type RelaxedLatencyStrategy struct{}

func (s *RelaxedLatencyStrategy) Validate(req LatencyRequest) error {
	if err := validateBasicLatencyInput(req); err != nil {
		return err
	}

	if req.LatencyMS > 250 {
		return fmt.Errorf("%w: %dms exceeds relaxed limit of 250ms",
			ErrLatencyTooHigh, req.LatencyMS)
	}
	if req.PacketLossPercent > 5 {
		return fmt.Errorf("%w: packet loss %.2f%% exceeds relaxed limit of 5%%",
			ErrPacketLossTooHigh, req.PacketLossPercent)
	}

	return nil
}

func validateBasicLatencyInput(req LatencyRequest) error {
	if req.LatencyMS < 0 || req.JitterMS < 0 ||
		req.PacketLossPercent < 0 || req.PacketLossPercent > 100 {
		return ErrInvalidInput
	}
	return nil
}

type LatencyValidator struct {
	strategy LatencyStrategy
}

func NewLatencyValidator(strategy LatencyStrategy) *LatencyValidator {
	return &LatencyValidator{strategy: strategy}
}

func (v *LatencyValidator) SetStrategy(strategy LatencyStrategy) {
	v.strategy = strategy
}

func (v *LatencyValidator) Validate(req LatencyRequest) error {
	if v.strategy == nil {
		return ErrUnknownStrategy
	}
	return v.strategy.Validate(req)
}

type LatencyStrategySelector struct {
	strategies map[ServiceTier]LatencyStrategy
}

func NewLatencyStrategySelector() *LatencyStrategySelector {
	return &LatencyStrategySelector{
		strategies: map[ServiceTier]LatencyStrategy{
			TierStrict:   &StrictLatencyStrategy{},
			TierStandard: &StandardLatencyStrategy{},
			TierRelaxed:  &RelaxedLatencyStrategy{},
		},
	}
}

func (s *LatencyStrategySelector) Choose(tier ServiceTier) (LatencyStrategy, error) {
	strategy, ok := s.strategies[tier]
	if !ok {
		return nil, ErrUnknownStrategy
	}
	return strategy, nil
}

func (s *LatencyStrategySelector) ChooseForRank(rank string) (LatencyStrategy, error) {
	switch rank {
	case "Eternus":
		return s.Choose(TierStrict)
	case "Ritualist":
		return s.Choose(TierStandard)
	case "Initiate":
		return s.Choose(TierRelaxed)
	}
	return nil, ErrUnknownStrategy
}