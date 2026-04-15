# go-sports-hub

Live sports data processing service built with Go.

## Stack

- **Go 1.22** — backend services, goroutines for concurrency
- **PostgreSQL 16** — persistent storage
- **Redis 7** — caching, pub/sub for live updates
- **NATS** — message queue for event processing pipeline
- **WebSocket** — real-time push to clients
- **Docker** — containerized deployment

## Architecture

```
External API → Fetcher → NATS → Processor → PostgreSQL
                                          → Redis (cache + pub/sub)
                                          → WebSocket clients
              REST API ← PostgreSQL / Redis
```

## Quick Start

```bash
docker compose up --build
```

API available at `http://localhost:8080`

## API

```
GET  /api/v1/events          list events (query: sport, league, status, limit, offset)
GET  /api/v1/events/live     live events (Redis-cached, 30s TTL)
GET  /api/v1/events/:id      event details with odds
GET  /api/v1/leagues         available leagues
GET  /api/v1/health          service health (postgres, redis, nats)
WS   /ws/events              live updates stream (query: sport, league)
```

## Demo Mode

Set `DEMO_MODE=true` to run with generated mock data — no external API key required.
The fetcher produces realistic football, basketball, and tennis events with live score
updates every 10 seconds.

## WebSocket

Connect to `ws://localhost:8080/ws/events` (optionally with `?sport=football&league=Premier+League`).

Send a JSON message to update your subscription filter at runtime:

```json
{"sport": "basketball", "league": "NBA"}
```
