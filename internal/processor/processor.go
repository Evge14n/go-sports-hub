package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Evge14n/go-sports-hub/internal/metrics"
	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/queue"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

type rawPayload struct {
	Event models.SportEvent `json:"event"`
	Odds  []models.Odds     `json:"odds"`
}

type Processor struct {
	db    storage.EventStore
	cache storage.CacheStore
	nats  *queue.Client
	log   *slog.Logger

	mu   sync.Mutex
	seen map[string]time.Time
}

func New(db storage.EventStore, cache storage.CacheStore, nats *queue.Client, log *slog.Logger) *Processor {
	return &Processor{
		db:    db,
		cache: cache,
		nats:  nats,
		log:   log,
		seen:  make(map[string]time.Time),
	}
}

func (p *Processor) Run(ctx context.Context) error {
	sub, err := p.nats.SubscribeRaw(queue.SubjectEventsRaw, func(msg *nats.Msg) {
		var payload rawPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			p.log.Error("unmarshal payload", "err", err)
			return
		}
		if err := p.process(ctx, payload); err != nil {
			p.log.Error("process event", "id", payload.Event.ID, "err", err)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe events.raw: %w", err)
	}

	go p.cleanupSeen(ctx)

	<-ctx.Done()
	sub.Unsubscribe()
	return nil
}

func (p *Processor) process(ctx context.Context, payload rawPayload) error {
	e := &payload.Event

	if p.isDuplicate(e.ID, e.UpdatedAt) {
		return nil
	}

	existing, err := p.db.GetEventByID(ctx, e.ID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}

	if existing == nil {
		if err := p.db.CreateEvent(ctx, e); err != nil {
			return fmt.Errorf("create event: %w", err)
		}
	} else {
		if err := p.db.UpdateEvent(ctx, e); err != nil {
			return fmt.Errorf("update event: %w", err)
		}
	}

	for i := range payload.Odds {
		if err := p.db.UpsertOdds(ctx, &payload.Odds[i]); err != nil {
			p.log.Warn("upsert odds", "id", payload.Odds[i].ID, "err", err)
		}
	}

	if err := p.cache.CacheEvent(ctx, e); err != nil {
		p.log.Warn("cache event", "id", e.ID, "err", err)
	}

	if e.Status == models.StatusLive {
		if err := p.cache.PublishUpdate(ctx, e); err != nil {
			p.log.Warn("publish update", "id", e.ID, "err", err)
		}
	}

	metrics.EventsProcessed.WithLabelValues(e.Sport, e.Status).Inc()
	return nil
}

func (p *Processor) isDuplicate(id string, updatedAt time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if last, ok := p.seen[id]; ok && !updatedAt.After(last) {
		return true
	}
	p.seen[id] = updatedAt
	return false
}

func (p *Processor) cleanupSeen(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-10 * time.Minute)
			p.mu.Lock()
			for id, t := range p.seen {
				if t.Before(cutoff) {
					delete(p.seen, id)
				}
			}
			p.mu.Unlock()
		}
	}
}
