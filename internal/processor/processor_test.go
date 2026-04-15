package processor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

type stubEventStore struct {
	events map[string]*models.SportEvent
	err    error
}

func newStubEventStore() *stubEventStore {
	return &stubEventStore{events: make(map[string]*models.SportEvent)}
}

func (s *stubEventStore) CreateEvent(_ context.Context, e *models.SportEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events[e.ID] = e
	return nil
}
func (s *stubEventStore) UpdateEvent(_ context.Context, e *models.SportEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events[e.ID] = e
	return nil
}
func (s *stubEventStore) GetEventByID(_ context.Context, id string) (*models.SportEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	e, ok := s.events[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}
func (s *stubEventStore) ListLiveEvents(_ context.Context) ([]models.SportEvent, error) {
	return nil, nil
}
func (s *stubEventStore) ListEvents(_ context.Context, _ storage.ListEventsFilter) ([]models.SportEvent, int, error) {
	return nil, 0, nil
}
func (s *stubEventStore) ListLeagues(_ context.Context) ([]string, error) { return nil, nil }
func (s *stubEventStore) UpsertOdds(_ context.Context, _ *models.Odds) error {
	return s.err
}
func (s *stubEventStore) GetOddsByEvent(_ context.Context, _ string) ([]models.Odds, error) {
	return nil, nil
}
func (s *stubEventStore) Ping(_ context.Context) error { return nil }

type stubCacheStore struct {
	cached    map[string]*models.SportEvent
	published []*models.SportEvent
	err       error
}

func newStubCacheStore() *stubCacheStore {
	return &stubCacheStore{cached: make(map[string]*models.SportEvent)}
}

func (s *stubCacheStore) CacheEvent(_ context.Context, e *models.SportEvent) error {
	if s.err != nil {
		return s.err
	}
	s.cached[e.ID] = e
	return nil
}
func (s *stubCacheStore) GetCachedEvent(_ context.Context, id string) (*models.SportEvent, error) {
	e, ok := s.cached[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}
func (s *stubCacheStore) CacheLiveEvents(_ context.Context, _ []models.SportEvent) error {
	return nil
}
func (s *stubCacheStore) GetCachedLiveEvents(_ context.Context) ([]models.SportEvent, error) {
	return nil, nil
}
func (s *stubCacheStore) PublishUpdate(_ context.Context, e *models.SportEvent) error {
	s.published = append(s.published, e)
	return nil
}
func (s *stubCacheStore) SubscribeUpdates(_ context.Context) <-chan *models.SportEvent {
	return make(chan *models.SportEvent)
}
func (s *stubCacheStore) Ping(_ context.Context) error { return nil }

func TestIsDuplicate_FirstSeen(t *testing.T) {
	p := &Processor{seen: make(map[string]time.Time)}
	if p.isDuplicate("ev-1", time.Now()) {
		t.Fatal("first event should not be duplicate")
	}
}

func TestIsDuplicate_SameTimestamp(t *testing.T) {
	p := &Processor{seen: make(map[string]time.Time)}
	ts := time.Now()
	p.isDuplicate("ev-1", ts)
	if !p.isDuplicate("ev-1", ts) {
		t.Fatal("same timestamp should be duplicate")
	}
}

func TestIsDuplicate_Newer(t *testing.T) {
	p := &Processor{seen: make(map[string]time.Time)}
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	p.isDuplicate("ev-1", t1)
	if p.isDuplicate("ev-1", t2) {
		t.Fatal("newer timestamp should not be duplicate")
	}
}

func TestProcess_NewEventCreates(t *testing.T) {
	db := newStubEventStore()
	cache := newStubCacheStore()
	p := &Processor{
		db:    db,
		cache: cache,
		log:   testLogger(),
		seen:  make(map[string]time.Time),
	}

	ev := models.SportEvent{
		ID:        "new-1",
		Sport:     "football",
		Status:    models.StatusUpcoming,
		UpdatedAt: time.Now(),
	}
	err := p.process(context.Background(), rawPayload{Event: ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := db.events["new-1"]; !ok {
		t.Fatal("event should have been created")
	}
}

func TestProcess_ExistingEventUpdates(t *testing.T) {
	db := newStubEventStore()
	cache := newStubCacheStore()
	existing := &models.SportEvent{
		ID:        "ex-1",
		Sport:     "football",
		Status:    models.StatusUpcoming,
		HomeScore: 0,
		UpdatedAt: time.Now().Add(-time.Minute),
	}
	db.events["ex-1"] = existing

	p := &Processor{
		db:    db,
		cache: cache,
		log:   testLogger(),
		seen:  make(map[string]time.Time),
	}

	updated := models.SportEvent{
		ID:        "ex-1",
		Sport:     "football",
		Status:    models.StatusLive,
		HomeScore: 1,
		UpdatedAt: time.Now(),
	}
	err := p.process(context.Background(), rawPayload{Event: updated})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.events["ex-1"].HomeScore != 1 {
		t.Fatal("event should have been updated")
	}
}

func TestProcess_LiveEventPublishes(t *testing.T) {
	db := newStubEventStore()
	cache := newStubCacheStore()
	p := &Processor{
		db:    db,
		cache: cache,
		log:   testLogger(),
		seen:  make(map[string]time.Time),
	}

	ev := models.SportEvent{
		ID:        "live-1",
		Sport:     "football",
		Status:    models.StatusLive,
		UpdatedAt: time.Now(),
	}
	err := p.process(context.Background(), rawPayload{Event: ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cache.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(cache.published))
	}
}

func TestProcess_UpcomingEventDoesNotPublish(t *testing.T) {
	db := newStubEventStore()
	cache := newStubCacheStore()
	p := &Processor{
		db:    db,
		cache: cache,
		log:   testLogger(),
		seen:  make(map[string]time.Time),
	}

	ev := models.SportEvent{
		ID:        "up-1",
		Sport:     "tennis",
		Status:    models.StatusUpcoming,
		UpdatedAt: time.Now(),
	}
	err := p.process(context.Background(), rawPayload{Event: ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cache.published) != 0 {
		t.Fatal("upcoming event should not be published")
	}
}

func TestProcess_DBError(t *testing.T) {
	db := newStubEventStore()
	db.err = errors.New("db down")
	cache := newStubCacheStore()
	p := &Processor{
		db:    db,
		cache: cache,
		log:   testLogger(),
		seen:  make(map[string]time.Time),
	}

	ev := models.SportEvent{
		ID:        "err-1",
		Sport:     "football",
		Status:    models.StatusLive,
		UpdatedAt: time.Now(),
	}
	err := p.process(context.Background(), rawPayload{Event: ev})
	if err == nil {
		t.Fatal("expected error from db")
	}
}

func testLogger() *slog.Logger {
	return slog.Default()
}
