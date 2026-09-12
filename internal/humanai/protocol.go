// Package humanai defines the future local bridge boundary. No provider calls or
// game commands are implemented. Providers must never directly control Recoil.
package humanai

import "context"

const ProtocolVersion = 1

type Observation struct {
	Version int    `json:"version"`
	MatchID string `json:"match_id"`
	Frame   int64  `json:"frame"`
	TeamID  int    `json:"team_id"`
	// Only state visible to this team belongs here; no hidden opponent information.
	Units []Unit `json:"units"`
}
type Unit struct {
	ID   int     `json:"id"`
	Kind string  `json:"kind"`
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
}
type StrategyPlan struct {
	Version      int      `json:"version"`
	MatchID      string   `json:"match_id"`
	BasedOnFrame int64    `json:"based_on_frame"`
	Intents      []Intent `json:"intents"`
}
type Intent struct {
	Kind    string  `json:"kind"`
	UnitIDs []int   `json:"unit_ids"`
	TargetX float64 `json:"target_x"`
	TargetZ float64 `json:"target_z"`
}

// Provider adapters (Claude/Gemini first) translate the protocol into API requests.
// Credentials will be supplied locally via a separate BYOK credential store.
type Provider interface {
	Plan(context.Context, Observation) (StrategyPlan, error)
}

// Bridge validates team ownership, intent allowlists, and staleness before apply.
type Bridge interface {
	Observe(context.Context) (Observation, error)
	Apply(context.Context, StrategyPlan) error
}
