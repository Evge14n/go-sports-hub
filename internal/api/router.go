package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handlers, ws *WSHub) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

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
