package fetcher

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/queue"
)

type Fetcher struct {
	nats     *queue.Client
	apiKey   string
	demoMode bool
	log      *slog.Logger
}

func New(nats *queue.Client, apiKey string, demoMode bool, log *slog.Logger) *Fetcher {
	return &Fetcher{
		nats:     nats,
		apiKey:   apiKey,
		demoMode: demoMode,
		log:      log,
	}
}

func (f *Fetcher) Run(ctx context.Context) {
	liveTimer := time.NewTicker(10 * time.Second)
	upcomingTimer := time.NewTicker(60 * time.Second)
	defer liveTimer.Stop()
	defer upcomingTimer.Stop()

	f.fetchAndPublish(ctx, models.StatusLive)
	f.fetchAndPublish(ctx, models.StatusUpcoming)

	for {
		select {
		case <-ctx.Done():
			return
		case <-liveTimer.C:
			f.fetchAndPublish(ctx, models.StatusLive)
		case <-upcomingTimer.C:
			f.fetchAndPublish(ctx, models.StatusUpcoming)
		}
	}
}

func (f *Fetcher) fetchAndPublish(ctx context.Context, status string) {
	var events []models.SportEvent
	var odds []models.Odds
	var err error

	if f.demoMode {
		events, odds = generateDemoData(status)
	} else {
		events, odds, err = f.fetchFromAPI(ctx, status)
		if err != nil {
			f.log.Error("fetch from api", "status", status, "err", err)
			return
		}
	}

	for i := range events {
		payload := struct {
			Event models.SportEvent `json:"event"`
			Odds  []models.Odds     `json:"odds"`
		}{
			Event: events[i],
			Odds:  filterOddsByEvent(odds, events[i].ID),
		}

		if err := f.nats.Publish(queue.SubjectEventsRaw, payload); err != nil {
			f.log.Error("publish event", "id", events[i].ID, "err", err)
		}
	}

	f.log.Info("fetched events", "status", status, "count", len(events))
}

func (f *Fetcher) fetchFromAPI(ctx context.Context, status string) ([]models.SportEvent, []models.Odds, error) {
	_ = ctx
	return nil, nil, fmt.Errorf("external api not configured")
}

func filterOddsByEvent(odds []models.Odds, eventID string) []models.Odds {
	var out []models.Odds
	for _, o := range odds {
		if o.EventID == eventID {
			out = append(out, o)
		}
	}
	return out
}

var (
	footballLeagues = []string{"Premier League", "La Liga", "Bundesliga", "Serie A", "Ligue 1"}
	basketballLeagues = []string{"NBA", "EuroLeague", "NCAA"}
	tennisLeagues     = []string{"ATP Masters", "WTA Finals", "Grand Slam"}

	footballTeams = []string{
		"Arsenal", "Chelsea", "Liverpool", "Manchester City", "Manchester United",
		"Tottenham", "Real Madrid", "Barcelona", "Atletico Madrid", "Bayern Munich",
		"Borussia Dortmund", "Juventus", "AC Milan", "Inter Milan", "PSG",
	}
	basketballTeams = []string{
		"LA Lakers", "Golden State Warriors", "Chicago Bulls", "Boston Celtics",
		"Miami Heat", "Brooklyn Nets", "Dallas Mavericks", "Phoenix Suns",
		"Milwaukee Bucks", "Denver Nuggets",
	}
	tennisPlayers = []string{
		"Djokovic N.", "Alcaraz C.", "Sinner J.", "Medvedev D.",
		"Zverev A.", "Swiatek I.", "Sabalenka A.", "Gauff C.", "Rybakina E.",
	}
)

var demoState = make(map[string]*models.SportEvent)

func generateDemoData(status string) ([]models.SportEvent, []models.Odds) {
	now := time.Now().UTC()
	var events []models.SportEvent
	var odds []models.Odds

	sports := []struct {
		sport   string
		leagues []string
		teams   []string
		count   int
	}{
		{"football", footballLeagues, footballTeams, 3},
		{"basketball", basketballLeagues, basketballTeams, 2},
		{"tennis", tennisLeagues, tennisPlayers, 2},
	}

	for _, s := range sports {
		league := s.leagues[rand.Intn(len(s.leagues))]
		for i := 0; i < s.count; i++ {
			home, away := pickTwo(s.teams)
			id := fmt.Sprintf("%s-%s-%s-%d", s.sport, slug(home), slug(away), startOfDay(now))

			if status == models.StatusLive {
				e := updateOrCreate(id, s.sport, league, home, away, now, status)
				events = append(events, *e)
				odds = append(odds, generateOdds(id, e.HomeScore, e.AwayScore)...)
			} else {
				startTime := now.Add(time.Duration(24+rand.Intn(72)) * time.Hour)
				e := &models.SportEvent{
					ID:        fmt.Sprintf("%s-%s-%s-%d", s.sport, slug(home), slug(away), startTime.Unix()),
					Sport:     s.sport,
					League:    league,
					HomeTeam:  home,
					AwayTeam:  away,
					StartTime: startTime,
					Status:    models.StatusUpcoming,
					UpdatedAt: now,
					CreatedAt: now,
				}
				events = append(events, *e)
				odds = append(odds, generateOdds(e.ID, 0, 0)...)
			}
		}
	}

	return events, odds
}

func updateOrCreate(id, sport, league, home, away string, now time.Time, status string) *models.SportEvent {
	if e, ok := demoState[id]; ok {
		if rand.Float32() < 0.3 {
			e.HomeScore += rand.Intn(2)
		}
		if rand.Float32() < 0.3 {
			e.AwayScore += rand.Intn(2)
		}
		e.UpdatedAt = now
		return e
	}

	e := &models.SportEvent{
		ID:        id,
		Sport:     sport,
		League:    league,
		HomeTeam:  home,
		AwayTeam:  away,
		StartTime: now.Add(-time.Duration(rand.Intn(80)) * time.Minute),
		Status:    status,
		HomeScore: rand.Intn(3),
		AwayScore: rand.Intn(3),
		UpdatedAt: now,
		CreatedAt: now,
	}
	demoState[id] = e
	return e
}

func generateOdds(eventID string, homeScore, awayScore int) []models.Odds {
	now := time.Now().UTC()

	diff := float64(homeScore - awayScore)
	homeWin := clamp(1.5+rand.Float64()*2.0-diff*0.15, 1.05, 15.0)
	draw := clamp(3.0+rand.Float64()*1.0, 2.5, 5.0)
	awayWin := clamp(1.5+rand.Float64()*2.0+diff*0.15, 1.05, 15.0)

	return []models.Odds{
		{ID: eventID + ":1x2:1", EventID: eventID, Market: models.Market1X2, Outcome: "1", Price: round2(homeWin), UpdatedAt: now},
		{ID: eventID + ":1x2:X", EventID: eventID, Market: models.Market1X2, Outcome: "X", Price: round2(draw), UpdatedAt: now},
		{ID: eventID + ":1x2:2", EventID: eventID, Market: models.Market1X2, Outcome: "2", Price: round2(awayWin), UpdatedAt: now},
		{ID: eventID + ":ou:over", EventID: eventID, Market: models.MarketOverUnder, Outcome: "over_2.5", Price: round2(1.7 + rand.Float64()*0.5), UpdatedAt: now},
		{ID: eventID + ":ou:under", EventID: eventID, Market: models.MarketOverUnder, Outcome: "under_2.5", Price: round2(1.9 + rand.Float64()*0.5), UpdatedAt: now},
	}
}

func pickTwo(items []string) (string, string) {
	i := rand.Intn(len(items))
	j := rand.Intn(len(items) - 1)
	if j >= i {
		j++
	}
	return items[i], items[j]
}

func slug(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' {
			out = append(out, '_')
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		}
	}
	return string(out)
}

func startOfDay(t time.Time) int64 {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
