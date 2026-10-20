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

	"beeline-games/internal/core/collector"
	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
)

//go:embed sdk.js
var sdkJS string

var headRe = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)
var typeRe = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

type GamesProvider interface {
	GetGame(ctx context.Context, userID, gameID string) (domain.Game, error)
	IsPublished(ctx context.Context, gameID string) (bool, error)
}

type CollectorProvider interface {
	Add(e collector.Event)
}

type EventIn struct {
	Type		string		`json:"type"`
	Result		string		`json:"result"`
	Score		*float64	`json:"score"`
	DurationMs	*int64		`json:"duration_ms"`
	Payload		json.RawMessage	`json:"payload"`
}

type BatchIn struct {
	GameID		string		`json:"game_id"`
	SessionID	string		`json:"session_id"`
	PlayerID	string		`json:"player_id"`
	Events		[]EventIn	`json:"events"`
}

type Service interface {
	GetPlayableHTML(ctx context.Context, userID, gameID string) (string, bool, error)
	ProcessEvents(ctx context.Context, batch BatchIn) error
}

type service struct {
	games		GamesProvider
	collector	CollectorProvider
	publicBaseURL	string

	pubMu		sync.RWMutex
	pubCache	map[string]pubEntry
}

type pubEntry struct {
	ok	bool
	exp	time.Time
}

func NewService(games GamesProvider, col CollectorProvider, publicBaseURL string) Service {
	return &service{
		games:		games,
		collector:	col,
		publicBaseURL:	publicBaseURL,
		pubCache:	make(map[string]pubEntry),
	}
}

func (s *service) GetPlayableHTML(ctx context.Context, userID, gameID string) (string, bool, error) {
	g, err := s.games.GetGame(ctx, userID, gameID)
	if err != nil {
		return "", false, err
	}
	if g.HTML == "" {
		return "", false, coreErrs.ErrNotFound
	}

	if !g.Published && g.OwnerID != userID {
		return "", false, coreErrs.ErrNotFound
	}

	html := s.injectSDK(g.HTML, g.ID, g.Published)
	return html, g.Published, nil
}

func (s *service) injectSDK(html, id string, enabled bool) string {
	js := strings.NewReplacer(
		"__GAME_ID__", id,
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

func (s *service) ProcessEvents(ctx context.Context, batch BatchIn) error {
	if len(batch.SessionID) == 0 || len(batch.SessionID) > 64 || len(batch.Events) == 0 || len(batch.Events) > 50 {
		return coreErrs.ErrInvalidInput
	}

	if !s.isPublished(batch.GameID) {
		return nil
	}

	if len(batch.PlayerID) > 64 {
		batch.PlayerID = batch.PlayerID[:64]
	}

	for _, e := range batch.Events {
		if !typeRe.MatchString(e.Type) {
			continue
		}
		ev := collector.Event{
			GameID:		batch.GameID,
			SessionID:	batch.SessionID,
			PlayerID:	batch.PlayerID,
			Type:		e.Type,
			Score:		e.Score,
			DurationMs:	e.DurationMs,
		}
		if e.Result == "win" || e.Result == "lose" || e.Result == "quit" {
			ev.Result = e.Result
		}
		if len(e.Payload) > 0 && len(e.Payload) <= 2048 && string(e.Payload) != "null" {
			ev.Payload = e.Payload
		}
		s.collector.Add(ev)
	}
	return nil
}

func (s *service) isPublished(id string) bool {
	now := time.Now()
	s.pubMu.RLock()
	e, ok := s.pubCache[id]
	s.pubMu.RUnlock()

	if ok && now.Before(e.exp) {
		return e.ok
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p, err := s.games.IsPublished(ctx, id)
	if err != nil {
		return e.ok
	}

	s.pubMu.Lock()
	if len(s.pubCache) > 50000 {
		s.pubCache = make(map[string]pubEntry)
	}
	s.pubCache[id] = pubEntry{ok: p, exp: time.Now().Add(30 * time.Second)}
	s.pubMu.Unlock()

	return p
}
