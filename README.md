classDiagram
    direction TB

    class ServerSelectionStrategy {
        <<interface>>
        +Select(req SelectionRequest) Server
    }

    class NearestRegion {
        +Select(req SelectionRequest) Server
    }

    class LowestLoadStrategy {
        -loadStore ServerLoadStore
        +Select(req SelectionRequest) Server
    }

    class RoundRobinStrategy {
        -nextIndex int
        +Select(req SelectionRequest) Server
    }

    class ServerSelector {
        -strategy ServerSelectionStrategy
        +SetStrategy(strategy)
        +Select(req SelectionRequest) Server
    }

    class ServerStrategySelector {
        -strategies map
        +Register(mode, strategy)
        +Choose(mode) ServerSelectionStrategy
    }

    class ServerLoadStore {
        <<interface>>
        +CountActiveAllocations(serverID) int
    }

    class SQLiteServerLoadStore {
        -db DB
        +CountActiveAllocations(serverID) int
    }

    class LatencyStrategy {
        <<interface>>
        +Validate(req LatencyRequest) error
    }

    class StrictLatencyStrategy {
        +Validate(req LatencyRequest) error
    }

    class StandardLatencyStrategy {
        +Validate(req LatencyRequest) error
    }

    class RelaxedLatencyStrategy {
        +Validate(req LatencyRequest) error
    }

    class LatencyValidator {
        -strategy LatencyStrategy
        +SetStrategy(strategy)
        +Validate(req LatencyRequest) error
    }

    class LatencyStrategySelector {
        -strategies map
        +Choose(tier) LatencyStrategy
    }

    class ServerAllocator {
        -strategySelector ServerStrategySelector
        -latencyValidator LatencyValidator
        +Process(claimID, region, latencyMS) error
    }

    ServerSelectionStrategy <|.. NearestRegion
    ServerSelectionStrategy <|.. LowestLoadStrategy
    ServerSelectionStrategy <|.. RoundRobinStrategy

    ServerSelector o--> ServerSelectionStrategy : delegates to
    ServerStrategySelector ..> ServerSelectionStrategy : chooses
    LowestLoadStrategy --> ServerLoadStore : reads load
    ServerLoadStore <|.. SQLiteServerLoadStore
    ServerAllocator --> ServerStrategySelector : selects server algorithm
    ServerAllocator --> LatencyValidator : validates latency

    LatencyStrategy <|.. StrictLatencyStrategy
    LatencyStrategy <|.. StandardLatencyStrategy
    LatencyStrategy <|.. RelaxedLatencyStrategy

    LatencyValidator o--> LatencyStrategy : delegates to
    LatencyStrategySelector ..> LatencyStrategy : chooses