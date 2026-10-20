# Ревью для основного бэкендера (ветка `model`)

Моя зона — LLM, промпты, агент. Ниже то, что задевает остальной бэкенд.

## ⚠️ Ломающие изменения API (фронта ещё нет, но смоук устарел)
| Было | Стало |
|---|---|
| `POST /api/games {prompt}` → сразу игра (синхронно) | → **план** `{game, plan, reply}`, `201`, статус `awaiting_approval`. `auto_approve:true` → сразу сборка, `202` |
| `POST /api/games/{id}/messages` → новая версия (синхронно) | → `202 {game}`, сборка в фоне; `409` если уже идёт |

Новые маршруты (`games/transport/http/pipeline.go`):
`POST /{id}/plan {content}` — правка плана · `POST /{id}/approve {spec?}` — запуск сборки (можно прислать отредактированный план) ·
`GET /{id}/status` · `GET /{id}/events?token=` (SSE) · `GET /{id}/generations` · `GET /{id}/preview.jpg` ·
`GET /api/generations/stats?days=` (manager) · `GET /api/benchmarks`.
Статусы игры: `planning → awaiting_approval → generating → validating → testing → fixing → ready | failed`.

## Что я поменял вне своей зоны — посмотри
- `play/service/service.go` — **баг**: не было `//go:embed sdk.js`, SDK вставлялся пустым → статистика не собиралась вообще.
- `Dockerfile` — **баг**: собирал `./cmd/server` (нет такого), исправил на `./cmd/app`.
- `docker-compose.yml` — сервис `browser` (chromedp/headless-shell) + переменные `LLM_*`, `BROWSER_WS_URL`, `OPENROUTER_URL`.
- `migrations/000002_agent_pipeline` — `games.status/status_detail/spec`, таблицы `generations`, `llm_calls`.
- `games/repository` — новые колонки в `gameCols`/scan; `ListOwnerGames` теперь отдаёт и черновики (убран фильтр `version>0`).
- `games/service` — вместо `inflight sync.Map` атомарный `TrySetStatus` в БД; фоновые задачи на контексте приложения, `Wait()` в graceful shutdown, `RecoverInterrupted()` при старте.
- `main.go` — wiring агента и тестера.
- `go.mod` — `chromedp v0.14.2` (v0.21 поменял API на generic — не обновлять вслепую).

## Нашёл, но не трогал
1. `internal/core/transport/http/server/server.go` — остаток монолита, импортирует несуществующие пакеты → `go build ./...` / `go vet ./...` падают. Никем не импортируется — **удалить**.
2. `GET /api/games/{id}`, `GET /g/{id}`, `preview.jpg` без auth-middleware → `userID` всегда пустой, владелец **не видит свои черновики**, хотя README обещает `?token=`. Нужен optional-auth middleware.
3. `scripts/smoke.sh` и README устарели (ждут синхронную генерацию; README про `cmd/server`, `lib/pq`, gzip, которых нет).
4. Нет rate-limit на `/api/games` и `/api/events`.

## Предложения
- `analytics.TopInsights` с фильтром по жанру/аудитории + «антипримеры» (низкий completion) — агент сразу станет точнее.
- Дашборд: сравнивать метрики с `/api/benchmarks`.
