package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	mw "github.com/Evge14n/go-sports-hub/internal/api/middleware"
	_ "github.com/Evge14n/go-sports-hub/internal/metrics"
)

func NewRouter(h *Handlers, ws *WSHub, log *slog.Logger, allowedOrigins string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(mw.CORS(allowedOrigins))
	r.Use(mw.RequestLogger(log))
	r.Use(mw.RateLimit(100, 200))
	r.Use(chimw.Recoverer)
	r.Use(mw.Metrics)

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/events", h.ListEvents)
		r.Get("/events/live", h.ListLiveEvents)
		r.Get("/events/{id}", h.GetEvent)
		r.Get("/leagues", h.ListLeagues)
		r.Get("/health", h.Health)
	})

	r.Get("/ws/events", ws.ServeHTTP)

	return r
}
