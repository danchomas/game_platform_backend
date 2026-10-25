# Ревью для основного бэкендера (ветка `model`)

Ветка `model` = текущий `main` + агентный конвейер генерации (моя зона: LLM, промпты, агент). Всё сделано под архитектуру `main`: порты у потребителя, DTO в транспорте, ошибки через `apperr`, `htmlguard` остаётся первым фильтром ответа модели.

## ⚠️ Меняется контракт API (фронт в этой ветке уже обновлён)
| Было | Стало |
|---|---|
| `POST /api/games {prompt}` → игра (синхронно) | → **план** `{game, plan, reply}`, `201`, статус `awaiting_approval`; `auto_approve:true` → `202` и сразу сборка |
| `POST /api/games/{id}/messages` → новая версия (синхронно) | → `202 {game}`, сборка в фоне |

Новые маршруты: `POST /{id}/plan`, `POST /{id}/approve {spec?}`, `GET /{id}/status`, `GET /{id}/events` (SSE), `GET /{id}/generations`, `GET /{id}/preview.jpg`, `GET /api/generations/stats` (менеджер), `GET /api/benchmarks`. Подробно — README, раздел «Генерация».

## Что задето вне моей зоны — посмотри
- `features/games` целиком: сервис стал конвейером (`pipeline.go`), `create_game.go`/`generate.go` удалены; одна сборка на игру — атомарный `TrySetStatus` в БД вместо `inflight sync.Map`; фоновые задачи на контексте приложения, `Wait()` в shutdown, `RecoverInterrupted()` при старте.
- `games/repository`: новые колонки в `gameCols`; `ListOwnerGames` отдаёт и черновики (убран `version>0`) — у плана ещё нет кода.
- `domain.Game`: `Status`, `StatusDetail`, `Spec`; `VersionInput.Instruction`.
- `core/llm`: `Client` получил `Complete` (модель на запрос, запасные модели, usage/стоимость, повтор 429/5xx); `Chat` и классификация ошибок прежние. Старые `System`/`BuildMessages` заменены промптами ролей.
- `config`: `LLM_PLANNER_MODEL`, `LLM_FIXER_MODEL`, `LLM_FALLBACK_MODELS`, `LLM_MAX_TOKENS`, `MAX_FIX_ROUNDS`, `BROWSER_*`; кодер по умолчанию `claude-haiku-5.5` (бюджет), `LLM_TIMEOUT_SEC` 240.
- `migrations/000002_agent_pipeline`, compose-сервис `browser` (chromedp/headless-shell), `frontend/nginx.conf` — `proxy_buffering off` для SSE.
- `go.mod`: `chromedp v0.14.2` (v0.21 сменил API на generic — не обновлять вслепую).
- `frontend/src/**/*.css`: шкала шрифтов поднята под монитор 1080p (было 7–10px на основном тексте, стало 13–15px; минимум 11px, в т.ч. в медиа-запросах для узких экранов); заголовки ≥ 26px не менялись. Боковое меню шире (272px), чтобы пункты не переносились. Проверено на 1920×1080, 1366×768 и 375×812: без обрезаний и горизонтальной прокрутки.
- `scripts/smoke.sh`: под план → сборку; проверка порядка SDK теперь ищет `data-gamekit` (GameKit сам создаёт `<canvas>`).

## Прогресс сборки (экран ожидания)
- `core/llm`: `Request.OnDelta` — ответ модели потоком (`stream: true`, SSE OpenRouter); без него всё как раньше. Ошибка посреди потока не повторяется (часть ответа уже показана). Шлюз Билайна должен уметь OpenAI-совместимый `stream`, иначе прогресс будет только по этапам.
- `GET /status` и SSE `/events` получили поле `progress` (только пока идёт сборка): `elapsed_ms`, `expected_ms`, `role`, `phase_ms`, `chars`, `expected_chars`, `step`. Хранится в памяти процесса, как и SSE-брокер.
- `GameService.GetStatus` возвращает ещё и прогресс; в `PipelineRepository` — `BuildEstimate` (медианы последних 20 удачных сборок).
- Миграция `000004`: `llm_calls.output_chars` — по ней оценивается объём следующих игр.
- Фронт: живой блок в чате (что пишется, полоса, прошло / осталось) и то же на заглушке предпросмотра; `tests/mock-api.mjs` имитирует постепенное написание кода.

## Проверено
`go build/vet`, `go test -race ./...`, 28 тестов фронта, полный стек в Docker с моком LLM: план → правка → подтверждение → тест в Chrome поймал ошибки → исправитель → v1 → правка → v2, `smoke.sh` целиком. **С реальной моделью не проверялось** — нет ключа.

## Предложения
- `analytics.TopInsights` с фильтром по жанру/аудитории — подсказки агенту станут точнее.
- Дашборд: сравнивать метрики с `/api/benchmarks`, показывать `/api/generations/stats`.
- SSE-брокер в памяти процесса: при нескольких репликах API нужен общий (Postgres LISTEN/NOTIFY).
