package attention

import "time"

// ReminderInput is the per-project input to the score calculation algorithm.
type ReminderInput struct {
	Waiting    float64   // max waiting duration across sessions, in minutes
	Streak     float64   // estimated continuous active minutes for this project
	Total      float64   // cumulative focus minutes today for this project
	LastActive time.Time // most recent session activity timestamp in this project
	IsPrimary  bool      // whether this project is the primary (主线)
}

// ReminderOutput is the per-project computed reminder result, serialized
// to the Dashboard API response under the reminder_scores field.
type ReminderOutput struct {
	// Raw algorithm values
	Score    float64 `json:"score"`     // raw reminder score (raw^EXP_POWER)
	FogScore float64 `json:"fog_score"` // raw fog suppression score [0,1]

	// 0-100 scale for frontend consumption
	Highlight int `json:"highlight"` // 0-100 highlight intensity (log-mapped from Score)
	FogPct    int `json:"fog_pct"`   // 0-100 fog opacity (FogScore * 100)

	// Metadata
	Level     string  `json:"level"`
	Waiting   float64 `json:"waiting_min"`
	Streak    float64 `json:"streak_min"`
	IsCurrent bool    `json:"is_current"`
}

// ProjectActivity tracks activity metrics for a project.
// Used internally during score computation.
type ProjectActivity struct {
	Path           string
	Streak         float64 // continuous active minutes
	Total          float64 // cumulative focus today
	LastActiveTime int64   // most recent activity unix timestamp
	IsPrimary      bool
}
