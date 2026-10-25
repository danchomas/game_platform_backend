package http

import (
	"encoding/json"

	"beeline-games/internal/features/play/service"
)

type eventRequest struct {
	Type       string          `json:"type"`
	Result     string          `json:"result"`
	Score      *float64        `json:"score"`
	DurationMs *int64          `json:"duration_ms"`
	Payload    json.RawMessage `json:"payload"`
}

type batchRequest struct {
	GameID    string         `json:"game_id"`
	SessionID string         `json:"session_id"`
	PlayerID  string         `json:"player_id"`
	Events    []eventRequest `json:"events"`
}

func (b batchRequest) toDomain() service.EventBatch {
	events := make([]service.Event, 0, len(b.Events))
	for _, e := range b.Events {
		events = append(events, service.Event{
			Type:       e.Type,
			Result:     e.Result,
			Score:      e.Score,
			DurationMs: e.DurationMs,
			Payload:    e.Payload,
		})
	}

	return service.EventBatch{
		GameID:    b.GameID,
		SessionID: b.SessionID,
		PlayerID:  b.PlayerID,
		Events:    events,
	}
}
