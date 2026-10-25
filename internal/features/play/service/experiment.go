package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/collector"
)

// eventExposure — показ варианта эксперимента. Пишется сервером в момент отдачи HTML:
// вариант, который падает до первого события SDK, иначе выглядел бы «без проблем».
const eventExposure = "exposure"

// playerIDRe — идентификатор игрока из cookie или ?uid=. Он вставляется в JS-строку SDK,
// поэтому набор символов закрыт: кавычка или </script> здесь — готовый XSS.
var playerIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// PlayExperiment отдаёт игроку его вариант эксперимента. playerID — из cookie или ?uid=;
// непригодный или пустой заменяется новым, и транспорт должен его запомнить.
func (s *Service) PlayExperiment(ctx context.Context, experimentID, playerID string) (html, effectivePlayerID string, err error) {
	if !playerIDRe.MatchString(playerID) {
		if playerID, err = randomID(); err != nil {
			return "", "", err
		}
	}

	gameID, err := s.experiments.PickVariant(ctx, experimentID, playerID)
	if err != nil {
		return "", "", err
	}
	g, err := s.games.GetGame(ctx, "", gameID)
	if err != nil {
		return "", "", err
	}
	if !g.Published || g.HTML == "" {
		return "", "", apperr.ErrNotFound
	}

	sessionID, err := randomID()
	if err != nil {
		return "", "", err
	}
	payload, err := json.Marshal(map[string]string{"experiment": experimentID})
	if err != nil {
		return "", "", fmt.Errorf("метка показа: %w", err)
	}
	s.collector.Add(collector.Event{
		GameID: g.ID, SessionID: sessionID, PlayerID: playerID, Type: eventExposure, Payload: payload,
	})

	html = s.injectSDK(g.HTML, sdkParams{GameID: g.ID, Enabled: true, PlayerID: playerID, SessionID: sessionID})
	return html, playerID, nil
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("генерация идентификатора: %w", err)
	}
	return hex.EncodeToString(b), nil
}
