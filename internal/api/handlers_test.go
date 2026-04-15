package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

type mockEventStore struct {
	createEvent  func(ctx context.Context, e *models.SportEvent) error
	updateEvent  func(ctx context.Context, e *models.SportEvent) error
	getEventByID func(ctx context.Context, id string) (*models.SportEvent, error)
	listLive     func(ctx context.Context) ([]models.SportEvent, error)
	listEvents   func(ctx context.Context, f storage.ListEventsFilter) ([]models.SportEvent, int, error)
	listLeagues  func(ctx context.Context) ([]string, error)
	upsertOdds   func(ctx context.Context, o *models.Odds) error
	getOdds      func(ctx context.Context, eventID string) ([]models.Odds, error)
	ping         func(ctx context.Context) error
}

func (m *mockEventStore) CreateEvent(ctx context.Context, e *models.SportEvent) error {
	return m.createEvent(ctx, e)
}
func (m *mockEventStore) UpdateEvent(ctx context.Context, e *models.SportEvent) error {
	return m.updateEvent(ctx, e)
}
func (m *mockEventStore) GetEventByID(ctx context.Context, id string) (*models.SportEvent, error) {
	return m.getEventByID(ctx, id)
}
func (m *mockEventStore) ListLiveEvents(ctx context.Context) ([]models.SportEvent, error) {
	return m.listLive(ctx)
}
func (m *mockEventStore) ListEvents(ctx context.Context, f storage.ListEventsFilter) ([]models.SportEvent, int, error) {
	return m.listEvents(ctx, f)
}
func (m *mockEventStore) ListLeagues(ctx context.Context) ([]string, error) {
	return m.listLeagues(ctx)
}
func (m *mockEventStore) UpsertOdds(ctx context.Context, o *models.Odds) error {
	return m.upsertOdds(ctx, o)
}
func (m *mockEventStore) GetOddsByEvent(ctx context.Context, eventID string) ([]models.Odds, error) {
	return m.getOdds(ctx, eventID)
}
func (m *mockEventStore) Ping(ctx context.Context) error {
	return m.ping(ctx)
}

type mockCacheStore struct {
	cacheEvent     func(ctx context.Context, e *models.SportEvent) error
	getCached      func(ctx context.Context, id string) (*models.SportEvent, error)
	cacheLive      func(ctx context.Context, events []models.SportEvent) error
	getCachedLive  func(ctx context.Context) ([]models.SportEvent, error)
	publishUpdate  func(ctx context.Context, e *models.SportEvent) error
	subUpdates     func(ctx context.Context) <-chan *models.SportEvent
	ping           func(ctx context.Context) error
}

func (m *mockCacheStore) CacheEvent(ctx context.Context, e *models.SportEvent) error {
	return m.cacheEvent(ctx, e)
}
func (m *mockCacheStore) GetCachedEvent(ctx context.Context, id string) (*models.SportEvent, error) {
	return m.getCached(ctx, id)
}
func (m *mockCacheStore) CacheLiveEvents(ctx context.Context, events []models.SportEvent) error {
	return m.cacheLive(ctx, events)
}
func (m *mockCacheStore) GetCachedLiveEvents(ctx context.Context) ([]models.SportEvent, error) {
	return m.getCachedLive(ctx)
}
func (m *mockCacheStore) PublishUpdate(ctx context.Context, e *models.SportEvent) error {
	return m.publishUpdate(ctx, e)
}
func (m *mockCacheStore) SubscribeUpdates(ctx context.Context) <-chan *models.SportEvent {
	return m.subUpdates(ctx)
}
func (m *mockCacheStore) Ping(ctx context.Context) error {
	return m.ping(ctx)
}

type mockNats struct {
	connected bool
}

func (m *mockNats) IsConnected() bool { return m.connected }

func newTestHandlers(db *mockEventStore, cache *mockCacheStore, nats *mockNats) *Handlers {
	log := slog.Default()
	return NewHandlers(db, cache, nats, log)
}

func sampleEvent() models.SportEvent {
	return models.SportEvent{
		ID:        "test-1",
		Sport:     "football",
		League:    "Premier League",
		HomeTeam:  "Arsenal",
		AwayTeam:  "Chelsea",
		StartTime: time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC),
		Status:    models.StatusLive,
		HomeScore: 2,
		AwayScore: 1,
		UpdatedAt: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
}

func TestListEvents_Success(t *testing.T) {
	events := []models.SportEvent{sampleEvent()}
	db := &mockEventStore{
		listEvents: func(_ context.Context, _ storage.ListEventsFilter) ([]models.SportEvent, int, error) {
			return events, 1, nil
		},
	}
	h := newTestHandlers(db, &mockCacheStore{}, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()
	h.ListEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["total"].(float64)) != 1 {
		t.Fatalf("expected total 1, got %v", resp["total"])
	}
}

func TestListEvents_WithFilters(t *testing.T) {
	var captured storage.ListEventsFilter
	db := &mockEventStore{
		listEvents: func(_ context.Context, f storage.ListEventsFilter) ([]models.SportEvent, int, error) {
			captured = f
			return nil, 0, nil
		},
	}
	h := newTestHandlers(db, &mockCacheStore{}, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?sport=football&league=NBA&status=live&limit=10&offset=5", nil)
	w := httptest.NewRecorder()
	h.ListEvents(w, req)

	if captured.Sport != "football" {
		t.Fatalf("expected sport football, got %s", captured.Sport)
	}
	if captured.League != "NBA" {
		t.Fatalf("expected league NBA, got %s", captured.League)
	}
	if captured.Status != "live" {
		t.Fatalf("expected status live, got %s", captured.Status)
	}
	if captured.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", captured.Limit)
	}
	if captured.Offset != 5 {
		t.Fatalf("expected offset 5, got %d", captured.Offset)
	}
}

func TestListEvents_Error(t *testing.T) {
	db := &mockEventStore{
		listEvents: func(_ context.Context, _ storage.ListEventsFilter) ([]models.SportEvent, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	h := newTestHandlers(db, &mockCacheStore{}, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()
	h.ListEvents(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestListLiveEvents_CacheHit(t *testing.T) {
	events := []models.SportEvent{sampleEvent()}
	cache := &mockCacheStore{
		getCachedLive: func(_ context.Context) ([]models.SportEvent, error) {
			return events, nil
		},
	}
	h := newTestHandlers(&mockEventStore{}, cache, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/live", nil)
	w := httptest.NewRecorder()
	h.ListLiveEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "cache" {
		t.Fatalf("expected source cache, got %v", resp["source"])
	}
}

func TestListLiveEvents_CacheMiss(t *testing.T) {
	events := []models.SportEvent{sampleEvent()}
	cache := &mockCacheStore{
		getCachedLive: func(_ context.Context) ([]models.SportEvent, error) {
			return nil, errors.New("miss")
		},
		cacheLive: func(_ context.Context, _ []models.SportEvent) error {
			return nil
		},
	}
	db := &mockEventStore{
		listLive: func(_ context.Context) ([]models.SportEvent, error) {
			return events, nil
		},
	}
	h := newTestHandlers(db, cache, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/live", nil)
	w := httptest.NewRecorder()
	h.ListLiveEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["source"] != "db" {
		t.Fatalf("expected source db, got %v", resp["source"])
	}
}

func TestGetEvent_Found(t *testing.T) {
	ev := sampleEvent()
	db := &mockEventStore{
		getEventByID: func(_ context.Context, id string) (*models.SportEvent, error) {
			return &ev, nil
		},
		getOdds: func(_ context.Context, _ string) ([]models.Odds, error) {
			return nil, nil
		},
	}
	cache := &mockCacheStore{
		getCached: func(_ context.Context, _ string) (*models.SportEvent, error) {
			return nil, errors.New("miss")
		},
	}
	h := newTestHandlers(db, cache, &mockNats{})

	r := chi.NewRouter()
	r.Get("/api/v1/events/{id}", h.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/test-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	db := &mockEventStore{
		getEventByID: func(_ context.Context, _ string) (*models.SportEvent, error) {
			return nil, nil
		},
	}
	cache := &mockCacheStore{
		getCached: func(_ context.Context, _ string) (*models.SportEvent, error) {
			return nil, errors.New("miss")
		},
	}
	h := newTestHandlers(db, cache, &mockNats{})

	r := chi.NewRouter()
	r.Get("/api/v1/events/{id}", h.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/unknown", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetEvent_CacheHit(t *testing.T) {
	ev := sampleEvent()
	db := &mockEventStore{
		getOdds: func(_ context.Context, _ string) ([]models.Odds, error) {
			return nil, nil
		},
	}
	cache := &mockCacheStore{
		getCached: func(_ context.Context, _ string) (*models.SportEvent, error) {
			return &ev, nil
		},
	}
	h := newTestHandlers(db, cache, &mockNats{})

	r := chi.NewRouter()
	r.Get("/api/v1/events/{id}", h.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/test-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListLeagues_Success(t *testing.T) {
	db := &mockEventStore{
		listLeagues: func(_ context.Context) ([]string, error) {
			return []string{"Premier League", "La Liga"}, nil
		},
	}
	h := newTestHandlers(db, &mockCacheStore{}, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leagues", nil)
	w := httptest.NewRecorder()
	h.ListLeagues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 leagues, got %d", len(data))
	}
}

func TestListLeagues_Error(t *testing.T) {
	db := &mockEventStore{
		listLeagues: func(_ context.Context) ([]string, error) {
			return nil, errors.New("db error")
		},
	}
	h := newTestHandlers(db, &mockCacheStore{}, &mockNats{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leagues", nil)
	w := httptest.NewRecorder()
	h.ListLeagues(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHealth_OK(t *testing.T) {
	db := &mockEventStore{ping: func(_ context.Context) error { return nil }}
	cache := &mockCacheStore{ping: func(_ context.Context) error { return nil }}
	nats := &mockNats{connected: true}
	h := newTestHandlers(db, cache, nats)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
}

func TestHealth_Degraded(t *testing.T) {
	db := &mockEventStore{ping: func(_ context.Context) error { return errors.New("down") }}
	cache := &mockCacheStore{ping: func(_ context.Context) error { return nil }}
	nats := &mockNats{connected: true}
	h := newTestHandlers(db, cache, nats)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %v", resp["status"])
	}
}
