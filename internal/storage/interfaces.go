package storage

import (
	"context"

	"github.com/Evge14n/go-sports-hub/internal/models"
)

type EventStore interface {
	CreateEvent(ctx context.Context, e *models.SportEvent) error
	UpdateEvent(ctx context.Context, e *models.SportEvent) error
	GetEventByID(ctx context.Context, id string) (*models.SportEvent, error)
	ListLiveEvents(ctx context.Context) ([]models.SportEvent, error)
	ListEvents(ctx context.Context, f ListEventsFilter) ([]models.SportEvent, int, error)
	ListLeagues(ctx context.Context) ([]string, error)
	UpsertOdds(ctx context.Context, o *models.Odds) error
	GetOddsByEvent(ctx context.Context, eventID string) ([]models.Odds, error)
	Ping(ctx context.Context) error
}

type CacheStore interface {
	CacheEvent(ctx context.Context, e *models.SportEvent) error
	GetCachedEvent(ctx context.Context, id string) (*models.SportEvent, error)
	CacheLiveEvents(ctx context.Context, events []models.SportEvent) error
	GetCachedLiveEvents(ctx context.Context) ([]models.SportEvent, error)
	PublishUpdate(ctx context.Context, e *models.SportEvent) error
	SubscribeUpdates(ctx context.Context) <-chan *models.SportEvent
	Ping(ctx context.Context) error
}
