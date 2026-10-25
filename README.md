# Beeline Vibe Games — backend (Go)

Сервер платформы: чат с ИИ → готовая HTML5-игра → публикация по UUID → встраивание в WebView → аналитика для менеджера.

## Запуск

```bash
cp .env.example .env      # впишите OPENROUTER_API_KEY
docker compose up --build
# Фронтенд: http://localhost:3000   API: http://localhost:8080
BASE=http://localhost:8080 ./scripts/smoke.sh   # сквозная проверка, нужен живой ключ
```

Compose поднимает четыре контейнера: `db` (PostgreSQL), `migrator` (golang-migrate накатывает `migrations/`), `api` (стартует только после успешных миграций) и `frontend` (nginx со статикой React/Vite, проксирует `/api/`, `/g/` и `/healthz` на `api`).

Без Docker: `go run ./cmd/app` (нужны Postgres с применёнными миграциями и переменные из `.env`) и `npm run dev` в `./frontend`.

```bash
go build ./...
go test -race ./...
gofmt -l -w . && go vet ./...
```

## Настройки

| Переменная | По умолчанию | Что задаёт |
|---|---|---|
| `DATABASE_URL` | — (обязательна) | строка подключения к Postgres |
| `JWT_SECRET` | — (обязательна) | ключ подписи токенов |
| `OPENROUTER_API_KEY` | пусто | ключ OpenRouter; без него платформа работает, но генерация отвечает 503 |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai/api/v1` | **базовый** адрес OpenAI-совместимого API, `/chat/completions` дописывается сам; сюда же можно подставить локальный сервер (`http://ollama:11434/v1`) |
| `OPENROUTER_MODEL` | `anthropic/claude-sonnet-4.5` | любой id с openrouter.ai/models |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | внешний адрес: из него собираются ссылки на игру и адрес SDK |
| `MANAGER_CODE` | пусто | код для регистрации менеджером; пусто — без проверки |
| `MAX_CONCURRENT_GEN` | `8` | одновременных генераций на процесс |
| `LLM_TIMEOUT_SEC` | `180` | таймаут одного обращения к модели |
| `PORT` / `FRONTEND_PORT` | `8080` / `3000` | порты наружу в compose |

Без `DATABASE_URL` или `JWT_SECRET` процесс не стартует и пишет, чего не хватает. При старте в лог пишутся `base_url`, `model` и `key_set`.

## Архитектура

Feature-based, три слоя в каждой фиче: `repository` → `service` → `transport`.

```
cmd/app            DI вручную, graceful shutdown (дописывает буфер аналитики)
migrations/        схема в формате golang-migrate, применяется отдельным контейнером

internal/core/
  domain           сущности без json-тегов
  apperr           sentinel-ошибки и ValidationError
  config           чтение и проверка окружения
  jwt              HS256 на stdlib
  llm              клиент OpenRouter, системный промпт, разбор ответа
  htmlguard        проверка HTML от модели перед сохранением
  collector        буфер событий → пакетная запись через COPY
  repository/postgres/pool/pgx
  transport/http/  middleware (Auth, OptionalAuth), request, response

internal/features/
  auth  games  play  analytics      каждая: repository/postgres, service, transport/http

frontend/          студия, каталог и дашборд (React + Vite, nginx)
```

- Интерфейсы объявляет потребитель: `service/ports.go` описывает, что сервису нужно от хранилища, `transport/http/ports.go` — что транспорту нужно от сервиса. Сервис не импортирует `repository/postgres`.
- Наружу уходят DTO транспортного слоя, а не доменные структуры.
- Статус ответа выбирается в одном месте — `classify()` в `core/transport/http/response`.
- Роутер — stdlib `http.ServeMux` с паттернами Go 1.22, драйвер БД — `pgx/v5`.

Почему быстро:
- `POST /api/events` кладёт событие в канал и сразу отвечает `204`; коллектор сбрасывает пачки (500 мс / 1000 событий) одним `COPY`, счётчик `plays` обновляется той же транзакцией.
- Статус «игра опубликована» кэшируется на 30 с — приём события не ходит в БД.
- Запросы дашборда выполняются параллельно.
- Узкое место — LLM: генерации идут под семафором `MAX_CONCURRENT_GEN` (ожидание слота до 20 с, иначе 503), на одну игру — одна генерация за раз (иначе 409).

### Генерация

1. Промпт 3–4000 символов, иначе `400`.
2. Модель получает историю чата игры, текущий HTML и подсказку из аналитики.
3. Ответ проходит `htmlguard`: размер 256 B … 2 MB, наличие разметки игры, никаких внешних `src=`/`href=`/`url(` — игра обязана быть самодостаточной.
4. Непригодный ответ получает одну повторную попытку; сетевая ошибка — нет.
5. Версия, сообщения и карточка игры сохраняются одной транзакцией. Неудачная первая генерация удаляет созданную карточку.

### Доступ к модели

Клиент ходит в OpenRouter напрямую: обычный `http.Client`, без прокси и с проверкой сертификата.

| Что случилось | Ответ API |
|---|---|
| сеть, таймаут шлюза, не-JSON, не-200, ключ не задан | `503 llm_unavailable` |
| пустой ответ, HTML не прошёл проверку | `502 llm_failed` |
| отмена клиентом / истёкший дедлайн | `499` / `504` |

## Как это работает

1. Пользователь регистрируется по имени → получает `token` (JWT) и `access_key` (код для входа с другого устройства).
2. `POST /api/games {prompt}` → модель генерирует игру → она сохраняется под UUID как версия 1.
3. Чат: `POST /api/games/{id}/messages {content}` — «сделай кнопки больше» → модель возвращает новую полную версию. Все версии хранятся целиком, откат создаёт новую версию из старого текста.
4. `publish` → игра появляется в галерее и доступна всем по `GET /g/{id}` (это и есть URL для WebView).
5. При отдаче `/g/{id}` сервер вставляет `sdk.js` первым скриптом в `<head>`. Игра вызывает `GameSDK.start() / end(result, score) / event(name, data)`; SDK сам шлёт `session_start`, `engaged`, `session_end`, `js_error`.
6. В системный промпт подмешиваются самые «залипательные» опубликованные игры за 30 дней (жанр, аудитория, длительность сессии, win rate), подсказка кэшируется на 5 минут.

### Встраивание в приложение Билайна

```
https://<host>/g/<UUID>?uid=<идентификатор_пользователя_приложения>
```

`uid` (опционально) станет `player_id` в аналитике. Игра также шлёт события хосту через `window.parent.postMessage` и `ReactNativeWebView.postMessage` в формате `{source:"game-sdk", type:"start|end|event", data}` — можно ловить конец игры и начислять награды.
Автономный файл: `GET /api/games/{id}/export`.

## API

Авторизация: `Authorization: Bearer <token>` или `?token=<jwt>` (для `<iframe>` с черновиком).
Ошибки: `{"error": {"code": "...", "message": "...", "field": "..."}}`, `field` — только у ошибок валидации.

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/auth/register` | `{name, role?: "player"\|"manager", manager_code?}` → `{user, token, access_key}` |
| POST | `/api/auth/login` | `{access_key}` → `{user, token}` |
| GET | `/api/me` | текущий пользователь |
| POST | `/api/games` | `{prompt}` → `{game, reply, version}` (синхронно, ~20–90 с) |
| GET | `/api/games/mine` | мои игры |
| GET | `/api/games/{id}` | игра с html (опубликованная — всем, черновик — владельцу) |
| PATCH / DELETE | `/api/games/{id}` | `{title}` / удалить |
| GET / POST | `/api/games/{id}/messages` | история чата / `{content}` → новая версия |
| GET | `/api/games/{id}/versions` | список версий |
| POST | `/api/games/{id}/versions/{v}/restore` | откат (создаёт новую версию) |
| POST | `/api/games/{id}/publish` · `/unpublish` | опубликовать / снять |
| GET | `/api/games/{id}/export` | скачать готовый HTML |
| GET | `/api/gallery?sort=new\|popular&q=&limit=&offset=` | галерея (публично) |
| GET | `/g/{id}` | сама игра для iframe/WebView |
| POST | `/api/events` | приём событий SDK (публичный, `text/plain`, `204`) |
| GET | `/api/analytics/overview?days=30&scope=mine\|all` | дашборд: `summary, daily[], games[], custom_events[], session_duration[]`; `scope=all` — только менеджеры |
| GET | `/api/analytics/games/{id}?days=30` | то же по одной игре (владелец или менеджер) |
| GET | `/healthz` | живость сервера и базы |

Метрики `summary`: `sessions, players, returning_players, engaged_sessions, game_starts, game_ends, wins, losses, avg_session_sec, avg_score, max_score, engagement_rate, completion_rate, win_rate`.

## Что учесть

- События принимаются только для **опубликованных** игр — превью не засоряет статистику. Тип события — `^[a-z0-9_]{1,40}$`.
- Правка опубликованной игры сразу меняет «живую» версию (кэш `/g/{id}` — 30 с).
- Роль `manager` выдаётся при регистрации; в проде задайте `MANAGER_CODE`.
- Нет rate-limit на `/api/events` и `/api/games` — для продакшена поставьте перед сервисом nginx/Cloudflare.
- Генерация синхронная: `WriteTimeout` сервера = `LLM_TIMEOUT_SEC` + 30 с. Следующий шаг — SSE-стриминг ответа модели.
- Тестами покрыты `core/jwt`, `core/llm`, `core/htmlguard` и сервисы `games`, `play`, `analytics`; транспорт, репозитории и фронтенд — пока нет.
