package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Evge14n/go-sports-hub/internal/models"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Postgres) CreateEvent(ctx context.Context, e *models.SportEvent) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO sport_events (id, sport, league, home_team, away_team, start_time, status, home_score, away_score, updated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		e.ID, e.Sport, e.League, e.HomeTeam, e.AwayTeam,
		e.StartTime, e.Status, e.HomeScore, e.AwayScore, e.UpdatedAt, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

func (p *Postgres) UpdateEvent(ctx context.Context, e *models.SportEvent) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE sport_events
		SET status=$2, home_score=$3, away_score=$4, updated_at=$5
		WHERE id=$1`,
		e.ID, e.Status, e.HomeScore, e.AwayScore, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

func (p *Postgres) GetEventByID(ctx context.Context, id string) (*models.SportEvent, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, sport, league, home_team, away_team, start_time, status, home_score, away_score, updated_at, created_at
		FROM sport_events WHERE id=$1`, id)

	e := &models.SportEvent{}
	err := row.Scan(&e.ID, &e.Sport, &e.League, &e.HomeTeam, &e.AwayTeam,
		&e.StartTime, &e.Status, &e.HomeScore, &e.AwayScore, &e.UpdatedAt, &e.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get event by id: %w", err)
	}
	return e, nil
}

func (p *Postgres) ListLiveEvents(ctx context.Context) ([]models.SportEvent, error) {
	return p.listByFilter(ctx, "status=$1", models.StatusLive)
}

func (p *Postgres) ListByLeague(ctx context.Context, league string) ([]models.SportEvent, error) {
	return p.listByFilter(ctx, "league=$1", league)
}

type ListEventsFilter struct {
	Sport  string
	League string
	Status string
	Limit  int
	Offset int
}

func (p *Postgres) ListEvents(ctx context.Context, f ListEventsFilter) ([]models.SportEvent, int, error) {
	where := " WHERE 1=1"
	args := []any{}
	i := 1

	if f.Sport != "" {
		where += fmt.Sprintf(" AND sport=$%d", i)
		args = append(args, f.Sport)
		i++
	}
	if f.League != "" {
		where += fmt.Sprintf(" AND league=$%d", i)
		args = append(args, f.League)
		i++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND status=$%d", i)
		args = append(args, f.Status)
		i++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM sport_events" + where
	if err := p.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}

	query := `
		SELECT id, sport, league, home_team, away_team, start_time, status, home_score, away_score, updated_at, created_at
		FROM sport_events` + where + " ORDER BY start_time DESC"

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", i, i+1)
	dataArgs := append(args, limit, f.Offset)

	events, err := p.scanEvents(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (p *Postgres) ListLeagues(ctx context.Context) ([]string, error) {
	rows, err := p.pool.Query(ctx, `SELECT DISTINCT league FROM sport_events ORDER BY league`)
	if err != nil {
		return nil, fmt.Errorf("list leagues: %w", err)
	}
	defer rows.Close()

	var leagues []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, fmt.Errorf("scan league: %w", err)
		}
		leagues = append(leagues, l)
	}
	return leagues, rows.Err()
}

func (p *Postgres) UpsertOdds(ctx context.Context, o *models.Odds) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO odds (id, event_id, market, outcome, price, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id, market, outcome) DO UPDATE
		SET price=EXCLUDED.price, updated_at=EXCLUDED.updated_at`,
		o.ID, o.EventID, o.Market, o.Outcome, o.Price, o.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert odds: %w", err)
	}
	return nil
}

func (p *Postgres) GetOddsByEvent(ctx context.Context, eventID string) ([]models.Odds, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, event_id, market, outcome, price, updated_at
		FROM odds WHERE event_id=$1 ORDER BY market, outcome`, eventID)
	if err != nil {
		return nil, fmt.Errorf("get odds: %w", err)
	}
	defer rows.Close()

	var odds []models.Odds
	for rows.Next() {
		var o models.Odds
		if err := rows.Scan(&o.ID, &o.EventID, &o.Market, &o.Outcome, &o.Price, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan odds: %w", err)
		}
		odds = append(odds, o)
	}
	return odds, rows.Err()
}

func (p *Postgres) listByFilter(ctx context.Context, where string, arg any) ([]models.SportEvent, error) {
	query := `
		SELECT id, sport, league, home_team, away_team, start_time, status, home_score, away_score, updated_at, created_at
		FROM sport_events WHERE ` + where + ` ORDER BY start_time DESC LIMIT 100`
	return p.scanEvents(ctx, query, arg)
}

func (p *Postgres) scanEvents(ctx context.Context, query string, args ...any) ([]models.SportEvent, error) {
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []models.SportEvent
	for rows.Next() {
		var e models.SportEvent
		if err := rows.Scan(&e.ID, &e.Sport, &e.League, &e.HomeTeam, &e.AwayTeam,
			&e.StartTime, &e.Status, &e.HomeScore, &e.AwayScore, &e.UpdatedAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
