package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Evge14n/go-sports-hub/internal/metrics"
	"github.com/Evge14n/go-sports-hub/internal/models"
	"github.com/Evge14n/go-sports-hub/internal/storage"
)

type natsPinger interface {
	IsConnected() bool
}

type Handlers struct {
	db    storage.EventStore
	cache storage.CacheStore
	nats  natsPinger
	log   *slog.Logger
}

func NewHandlers(db storage.EventStore, cache storage.CacheStore, nats natsPinger, log *slog.Logger) *Handlers {
	return &Handlers{db: db, cache: cache, nats: nats, log: log}
}

func (h *Handlers) ListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	f := storage.ListEventsFilter{
		Sport:  q.Get("sport"),
		League: q.Get("league"),
		Status: q.Get("status"),
		Limit:  limit,
		Offset: offset,
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	events, total, err := h.db.ListEvents(r.Context(), f)
	if err != nil {
		h.log.Error("list events", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	if events == nil {
		events = []models.SportEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":   events,
		"count":  len(events),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handlers) ListLiveEvents(w http.ResponseWriter, r *http.Request) {
	cached, err := h.cache.GetCachedLiveEvents(r.Context())
	if err == nil && cached != nil {
		metrics.CacheHits.Inc()
		writeJSON(w, http.StatusOK, map[string]any{"data": cached, "count": len(cached), "source": "cache"})
		return
	}
	metrics.CacheMisses.Inc()

	events, err := h.db.ListLiveEvents(r.Context())
	if err != nil {
		h.log.Error("list live events", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list live events")
		return
	}

	if events == nil {
		events = []models.SportEvent{}
	}

	go func() {
		_ = h.cache.CacheLiveEvents(context.Background(), events)
	}()

	writeJSON(w, http.StatusOK, map[string]any{"data": events, "count": len(events), "source": "db"})
}

func (h *Handlers) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if cached, err := h.cache.GetCachedEvent(r.Context(), id); err == nil && cached != nil {
		metrics.CacheHits.Inc()
		odds, _ := h.db.GetOddsByEvent(r.Context(), id)
		writeJSON(w, http.StatusOK, models.EventWithOdds{SportEvent: *cached, Odds: odds})
		return
	}
	metrics.CacheMisses.Inc()

	event, err := h.db.GetEventByID(r.Context(), id)
	if err != nil {
		h.log.Error("get event", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get event")
		return
	}
	if event == nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	odds, err := h.db.GetOddsByEvent(r.Context(), id)
	if err != nil {
		h.log.Warn("get odds", "event_id", id, "err", err)
	}

	writeJSON(w, http.StatusOK, models.EventWithOdds{SportEvent: *event, Odds: odds})
}

func (h *Handlers) ListLeagues(w http.ResponseWriter, r *http.Request) {
	leagues, err := h.db.ListLeagues(r.Context())
	if err != nil {
		h.log.Error("list leagues", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list leagues")
		return
	}
	if leagues == nil {
		leagues = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": leagues})
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbOK := h.db.Ping(ctx) == nil
	redisOK := h.cache.Ping(ctx) == nil
	natsOK := h.nats.IsConnected()

	status := "ok"
	code := http.StatusOK
	if !dbOK || !redisOK || !natsOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status": status,
		"checks": map[string]bool{
			"postgres": dbOK,
			"redis":    redisOK,
			"nats":     natsOK,
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
