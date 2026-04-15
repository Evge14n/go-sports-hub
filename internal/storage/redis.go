package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Evge14n/go-sports-hub/internal/models"
)

const (
	ttlLive     = 30 * time.Second
	ttlUpcoming = 5 * time.Minute
	keyLive     = "events:live:list"
	chanUpdates = "events:live"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(addr string) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Redis{client: client}, nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) CacheEvent(ctx context.Context, e *models.SportEvent) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	ttl := ttlUpcoming
	if e.Status == models.StatusLive {
		ttl = ttlLive
	}

	key := fmt.Sprintf("event:%s", e.ID)
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache event: %w", err)
	}
	return nil
}

func (r *Redis) GetCachedEvent(ctx context.Context, id string) (*models.SportEvent, error) {
	key := fmt.Sprintf("event:%s", id)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get cached event: %w", err)
	}

	var e models.SportEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &e, nil
}

func (r *Redis) CacheLiveEvents(ctx context.Context, events []models.SportEvent) error {
	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("marshal live events: %w", err)
	}
	if err := r.client.Set(ctx, keyLive, data, ttlLive).Err(); err != nil {
		return fmt.Errorf("cache live events: %w", err)
	}
	return nil
}

func (r *Redis) GetCachedLiveEvents(ctx context.Context) ([]models.SportEvent, error) {
	data, err := r.client.Get(ctx, keyLive).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get cached live events: %w", err)
	}

	var events []models.SportEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("unmarshal live events: %w", err)
	}
	return events, nil
}

func (r *Redis) PublishUpdate(ctx context.Context, e *models.SportEvent) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}
	if err := r.client.Publish(ctx, chanUpdates, data).Err(); err != nil {
		return fmt.Errorf("publish update: %w", err)
	}
	return nil
}

func (r *Redis) SubscribeUpdates(ctx context.Context) <-chan *models.SportEvent {
	out := make(chan *models.SportEvent, 64)

	go func() {
		defer close(out)
		sub := r.client.Subscribe(ctx, chanUpdates)
		defer sub.Close()

		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var e models.SportEvent
				if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
					continue
				}
				select {
				case out <- &e:
				default:
				}
			}
		}
	}()

	return out
}
