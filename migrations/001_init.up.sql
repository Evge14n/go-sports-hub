CREATE TABLE sport_events (
    id          VARCHAR(64)  PRIMARY KEY,
    sport       VARCHAR(50)  NOT NULL,
    league      VARCHAR(100) NOT NULL,
    home_team   VARCHAR(100) NOT NULL,
    away_team   VARCHAR(100) NOT NULL,
    start_time  TIMESTAMPTZ  NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'upcoming',
    home_score  INT          NOT NULL DEFAULT 0,
    away_score  INT          NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_status ON sport_events(status);
CREATE INDEX idx_events_sport  ON sport_events(sport);
CREATE INDEX idx_events_start  ON sport_events(start_time);

CREATE TABLE odds (
    id          VARCHAR(64)    PRIMARY KEY,
    event_id    VARCHAR(64)    NOT NULL REFERENCES sport_events(id),
    market      VARCHAR(50)    NOT NULL,
    outcome     VARCHAR(50)    NOT NULL,
    price       DECIMAL(10,4)  NOT NULL,
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE (event_id, market, outcome)
);

CREATE INDEX idx_odds_event ON odds(event_id);
