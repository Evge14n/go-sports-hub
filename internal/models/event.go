package models

import "time"

type SportEvent struct {
	ID        string    `json:"id" db:"id"`
	Sport     string    `json:"sport" db:"sport"`
	League    string    `json:"league" db:"league"`
	HomeTeam  string    `json:"home_team" db:"home_team"`
	AwayTeam  string    `json:"away_team" db:"away_team"`
	StartTime time.Time `json:"start_time" db:"start_time"`
	Status    string    `json:"status" db:"status"`
	HomeScore int       `json:"home_score" db:"home_score"`
	AwayScore int       `json:"away_score" db:"away_score"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Odds struct {
	ID        string    `json:"id" db:"id"`
	EventID   string    `json:"event_id" db:"event_id"`
	Market    string    `json:"market" db:"market"`
	Outcome   string    `json:"outcome" db:"outcome"`
	Price     float64   `json:"price" db:"price"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type EventWithOdds struct {
	SportEvent
	Odds []Odds `json:"odds"`
}

const (
	StatusUpcoming = "upcoming"
	StatusLive     = "live"
	StatusFinished = "finished"
)

const (
	Market1X2       = "1x2"
	MarketOverUnder = "over_under"
	MarketHandicap  = "handicap"
)
