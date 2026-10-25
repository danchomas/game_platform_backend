package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/collector"
)

//go:embed sdk.js
var sdkJS string

var headRe = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)

var eventTypeRe = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

const (
	maxSessionIDLen = 64
	maxPlayerIDLen  = 64
	maxBatchEvents  = 50
	maxPayloadBytes = 2048

	publishedTTL = 30 * time.Second

	publishedCacheCap = 50_000

	publishedLookupTimeout = 2 * time.Second
)

var allowedResults = map[string]bool{"win": true, "lose": true, "quit": true}

type Event struct {
	Type       string
	Result     string
	Score      *float64
	DurationMs *int64
	Payload    json.RawMessage
}

type EventBatch struct {
	GameID    string
	SessionID string
	PlayerID  string
	Events    []Event
}

type Service struct {
	games         GamesProvider
	collector     CollectorProvider
	publicBaseURL string

	pubMu    sync.RWMutex
	pubCache map[string]pubEntry
}

type pubEntry struct {
	ok  bool
	exp time.Time
}

func NewService(games GamesProvider, col CollectorProvider, publicBaseURL string) *Service {
	return &Service{
		games:         games,
		collector:     col,
		publicBaseURL: publicBaseURL,
		pubCache:      make(map[string]pubEntry),
	}
}

func (s *Service) GetPlayableHTML(ctx context.Context, userID, gameID string) (string, bool, error) {
	g, err := s.games.GetGame(ctx, userID, gameID)
	if err != nil {
		return "", false, err
	}
	if g.HTML == "" {
		return "", false, apperr.ErrNotFound
	}
	if !g.Published && g.OwnerID != userID {
		return "", false, apperr.ErrNotFound
	}

	return s.injectSDK(g.HTML, g.ID, g.Published), g.Published, nil
}

func (s *Service) injectSDK(html, gameID string, enabled bool) string {
	js := strings.NewReplacer(
		"__GAME_ID__", gameID,
		"__ENDPOINT__", s.publicBaseURL+"/api/events",
		"__ENABLED__", strconv.FormatBool(enabled),
	).Replace(sdkJS)
	tag := "<script>" + js + "</script>"

	if loc := headRe.FindStringIndex(html); loc != nil {
		return html[:loc[1]] + tag + html[loc[1]:]
	}
	if i := strings.Index(strings.ToLower(html), "<body"); i >= 0 {
		return html[:i] + tag + html[i:]
	}
	return tag + html
}

func (s *Service) ProcessEvents(ctx context.Context, batch EventBatch) error {
	if err := validateBatch(batch); err != nil {
		return err
	}

	if !s.isPublished(ctx, batch.GameID) {
		return nil
	}

	playerID := truncate(batch.PlayerID, maxPlayerIDLen)

	for _, e := range batch.Events {
		if !eventTypeRe.MatchString(e.Type) {
			continue
		}

		ev := collector.Event{
			GameID:     batch.GameID,
			SessionID:  batch.SessionID,
			PlayerID:   playerID,
			Type:       e.Type,
			Score:      e.Score,
			DurationMs: e.DurationMs,
		}
		if allowedResults[e.Result] {
			ev.Result = e.Result
		}
		if len(e.Payload) > 0 && len(e.Payload) <= maxPayloadBytes && string(e.Payload) != "null" {
			ev.Payload = e.Payload
		}

		s.collector.Add(ev)
	}

	return nil
}

func validateBatch(b EventBatch) error {
	switch {
	case b.GameID == "":
		return apperr.NewValidationError("game_id", "идентификатор игры не указан")
	case b.SessionID == "" || len(b.SessionID) > maxSessionIDLen:
		return apperr.NewValidationError("session_id", "идентификатор сессии пуст или слишком длинный")
	case len(b.Events) == 0 || len(b.Events) > maxBatchEvents:
		return apperr.NewValidationError("events", "в пачке должно быть от 1 до 50 событий")
	}
	return nil
}

func (s *Service) isPublished(ctx context.Context, gameID string) bool {
	s.pubMu.RLock()
	e, ok := s.pubCache[gameID]
	s.pubMu.RUnlock()

	if ok && time.Now().Before(e.exp) {
		return e.ok
	}

	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishedLookupTimeout)
	defer cancel()

	published, err := s.games.IsPublished(lookupCtx, gameID)
	if err != nil {
		return e.ok
	}

	s.pubMu.Lock()
	if len(s.pubCache) > publishedCacheCap {
		s.pubCache = make(map[string]pubEntry)
	}
	s.pubCache[gameID] = pubEntry{ok: published, exp: time.Now().Add(publishedTTL)}
	s.pubMu.Unlock()

	return published
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
