# Beeline Vibe Games — backend (Go)

Сервер платформы: чат с ИИ → готовая HTML5-игра → публикация по UUID → встраивание в WebView → аналитика для менеджера.

## Запуск

```bash
cp .env.example .env      # впишите OPENROUTER_API_KEY
docker compose up --build
# API: http://localhost:8080   (фронтенд: положите сборку в ./web, index.html отдаётся с /)
BASE=http://localhost:8080 ./scripts/smoke.sh   # сквозная проверка
```

Без Docker: `go run ./cmd/server` (нужен Postgres, переменные — см. `internal/config`).

## Архитектура

```
cmd/server        точка входа, graceful shutdown (дописывает буфер аналитики)
internal/api      HTTP: хендлеры, middleware (recover, log, CORS, gzip), SDK (sdk.js)
internal/llm      клиент OpenRouter, системный промпт, разбор ответа модели
internal/store    Postgres: схема (embed-миграция), игры/версии/чат, SQL-агрегаты аналитики
internal/collector  буфер событий → пакетная запись через COPY
internal/auth     JWT HS256 (stdlib)
```

Почему быстро:
- Net/http (Go 1.22 mux), единственная зависимость — `lib/pq`.
- `POST /api/events` кладёт событие в канал и сразу отвечает `204`; писатель сбрасывает пачки (500 мс / 1000 событий) одним `COPY`. Счётчик `plays` обновляется той же транзакцией.
- Статус «игра опубликована» кэшируется на 30 с → приём события не ходит в БД.
- Запросы дашборда выполняются параллельно; gzip для HTML/JSON.
- Узкое место — LLM; число одновременных генераций ограничено (`MAX_CONCURRENT_GEN`), на одну игру — одна генерация за раз.

## Как это работает

1. Менеджер регистрируется по имени → получает `token` (JWT) и `access_key` (код для входа с другого устройства).
2. `POST /api/games {prompt}` → LLM (OpenRouter) генерирует игру → сохраняется в БД под UUID как версия 1.
3. Чат: `POST /api/games/{id}/messages {content}` — «сделай кнопки больше» → модель получает текущий код + историю, возвращает новую полную версию. Все версии хранятся, можно откатиться.
4. `publish` → игра появляется в галерее и доступна всем по `GET /g/{id}` (это и есть URL для WebView).
5. При отдаче `/g/{id}` сервер **автоматически встраивает SDK аналитики** перед кодом игры. Генерируемая игра вызывает `GameSDK.start() / end(result, score) / event(name, data)` (это прописано в системном промпте). Дополнительно SDK сам шлёт `session_start`, `engaged` (первое касание), `session_end` (активное время), `js_error`.
6. **Бонус «учёт аналитики»:** в системный промпт подмешиваются 3 самые «залипательные» опубликованные игры за 30 дней (жанр, аудитория, средняя длительность сессии, win rate).

### Встраивание в приложение Билайна

```
https://<host>/g/<UUID>?uid=<идентификатор_пользователя_приложения>
```
`uid` (опционально) станет `player_id` в аналитике. Игра также шлёт события хосту: `window.parent.postMessage` и `ReactNativeWebView.postMessage` в формате `{source:"game-sdk", type:"start|end|event", data}` — можно ловить конец игры и начислять награды.
Автономный файл: `GET /api/games/{id}/export`.

## API

Авторизация: `Authorization: Bearer <token>`. Ошибки: `{"error":"..."}`.

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/auth/register` | `{name, role?: "player"\|"manager", manager_code?}` → `{user, token, access_key}` |
| POST | `/api/auth/login` | `{access_key}` → `{user, token}` |
| GET | `/api/me` | текущий пользователь |
| POST | `/api/games` | `{prompt}` → `{game, reply, version}` (синхронно, ~20–90 с) |
| GET | `/api/games/mine` | мои игры |
| GET | `/api/games/{id}` | игра с html (опубликованные — всем, черновик — владельцу) |
| PATCH / DELETE | `/api/games/{id}` | `{title}` / удалить |
| GET / POST | `/api/games/{id}/messages` | история чата / `{content}` → новая версия |
| GET | `/api/games/{id}/versions` | список версий |
| POST | `/api/games/{id}/versions/{v}/restore` | откат (создаёт новую версию) |
| POST | `/api/games/{id}/publish` · `/unpublish` | опубликовать / снять |
| GET | `/api/games/{id}/export` | скачать готовый HTML |
| GET | `/api/gallery?sort=new\|popular&q=&limit=&offset=` | галерея (публично) |
| GET | `/g/{id}` | сама игра для iframe/WebView; черновик владельца: `?token=<jwt>` |
| POST | `/api/events` | приём событий SDK (публичный, `text/plain`, `204`) |
| GET | `/api/analytics/overview?days=30&scope=mine\|all` | дашборд: `summary, daily[], games[], custom_events[], session_duration[]`; `scope=all` — только менеджеры |
| GET | `/api/analytics/games/{id}?days=30` | то же по одной игре (владелец или менеджер) |
| GET | `/healthz` | живость + счётчик отброшенных событий |

Метрики `summary`: `sessions, players, returning_players, engaged_sessions, game_starts, game_ends, wins, losses, avg_session_sec, avg_score, max_score, engagement_rate, completion_rate, win_rate`.

## Что учесть

- События принимаются только для **опубликованных** игр (превью не засоряет статистику).
- Правка опубликованной игры сразу меняет «живую» версию (кэш `/g/{id}` — 30 с).
- Роль `manager` выдаётся при регистрации; в проде задайте `MANAGER_CODE`.
- Нет rate-limit на `/api/events` и `/api/games` — для продакшена поставить перед сервисом nginx/Cloudflare.
- Генерация синхронная (долгий HTTP-запрос, таймаут сервера = `LLM_TIMEOUT_SEC`+30 с). Следующий шаг — SSE-стриминг ответа OpenRouter.
- Модель меняется переменной `OPENROUTER_MODEL` (любой id с openrouter.ai/models).
