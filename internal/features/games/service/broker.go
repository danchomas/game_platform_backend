package service

import (
	"sync"

	"beeline-games/internal/core/domain"
)

// broker раздаёт смены статуса игры подписчикам (SSE). Только в памяти процесса.
type broker struct {
	mu   sync.Mutex
	subs map[string]map[chan domain.StatusEvent]struct{}
}

func newBroker() *broker {
	return &broker{subs: make(map[string]map[chan domain.StatusEvent]struct{})}
}

func (b *broker) publish(ev domain.StatusEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[ev.GameID] {
		select {
		case ch <- ev:
		default: // медленный подписчик пропускает промежуточный статус
		}
	}
}

func (b *broker) subscribe(gameID string) (<-chan domain.StatusEvent, func()) {
	ch := make(chan domain.StatusEvent, 16)
	b.mu.Lock()
	if b.subs[gameID] == nil {
		b.subs[gameID] = make(map[chan domain.StatusEvent]struct{})
	}
	b.subs[gameID][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs[gameID], ch)
			if len(b.subs[gameID]) == 0 {
				delete(b.subs, gameID)
			}
			b.mu.Unlock()
		})
	}
}
