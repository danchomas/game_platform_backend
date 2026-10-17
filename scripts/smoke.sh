#!/usr/bin/env bash
# Сквозной смоук-тест API: регистрация -> генерация -> итерация -> публикация -> игра -> события -> дашборд.
# Использование: BASE=http://localhost:8080 ./scripts/smoke.sh   (нужен настроенный OPENROUTER_API_KEY)
B=${BASE:-http://localhost:8080}
j() { python3 -c "import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))" "$1"; }

echo "== register manager"; R=$(curl -s -XPOST $B/api/auth/register -d '{"name":"Анна","role":"manager"}'); echo "$R" | cut -c1-160
TOK=$(echo "$R" | j "d['token']"); KEY=$(echo "$R" | j "d['access_key']")
echo "== login by access_key"; curl -s -XPOST $B/api/auth/login -d "{\"access_key\":\"$KEY\"}" | j "d['user']['name']"
A="Authorization: Bearer $TOK"

echo "== create game"; G=$(curl -s -m 200 -XPOST $B/api/games -H "$A" -d '{"prompt":"замути змейку для бабушки"}')
ID=$(echo "$G" | j "d['game']['id']"); echo "$G" | j "(d['reply'], d['version'], d['game']['title'], d['game']['play_url'])"
echo "== iterate"; curl -s -m 200 -XPOST $B/api/games/$ID/messages -H "$A" -d '{"content":"сделай кнопки больше"}' | j "(d['reply'], 'v'+str(d['version']))"
echo "== chat history"; curl -s $B/api/games/$ID/messages -H "$A" | j "[(m['role'],m['content'][:28]) for m in d]"
echo "== versions"; curl -s $B/api/games/$ID/versions -H "$A" | j "[v['version'] for v in d]"
echo "== draft hidden from anon (404)"; curl -s -o /dev/null -w "%{http_code}\n" $B/g/$ID
echo "== draft preview by owner token (200)"; curl -s -o /dev/null -w "%{http_code}\n" "$B/g/$ID?token=$TOK"
echo "== publish"; curl -s -XPOST $B/api/games/$ID/publish -H "$A" | j "d['published']"
echo "== public play: SDK injected before game code"; curl -s $B/g/$ID | python3 -c "import sys; h=sys.stdin.read(); print('GameSDK' in h and h.index('gsdk_pid') < h.index('<canvas'))"

echo "== player registers & plays (events)"
P=$(curl -s -XPOST $B/api/auth/register -d '{"name":"Игрок"}'); PTOK=$(echo "$P" | j "d['token']")
for s in s1 s2 s3; do
  curl -s -o /dev/null -w "%{http_code} " -XPOST $B/api/events -H 'Content-Type: text/plain' -d "{\"game_id\":\"$ID\",\"session_id\":\"$s\",\"player_id\":\"p-$s\",\"events\":[{\"type\":\"session_start\"},{\"type\":\"engaged\"},{\"type\":\"game_start\"},{\"type\":\"game_end\",\"result\":\"win\",\"score\":$((RANDOM%90+10))},{\"type\":\"bonus_taken\",\"payload\":{\"n\":1}},{\"type\":\"session_end\",\"duration_ms\":$((RANDOM%60000+5000))}]}"
done; echo
echo "== event for unknown game is ignored (204)"; curl -s -o /dev/null -w "%{http_code}\n" -XPOST $B/api/events -d "{\"game_id\":\"$(python3 -c 'import uuid;print(uuid.uuid4())')\",\"session_id\":\"x\",\"events\":[{\"type\":\"session_start\"}]}"
sleep 1.2
echo "== gallery (anon)"; curl -s $B/api/gallery | j "[(g['title'],g['owner_name'],g['plays']) for g in d]"
echo "== player cannot edit foreign game (403)"; curl -s -o /dev/null -w "%{http_code}\n" -XPOST $B/api/games/$ID/messages -H "Authorization: Bearer $PTOK" -d '{"content":"сломай"}'
echo "== player cannot see all-scope analytics (403)"; curl -s -o /dev/null -w "%{http_code}\n" "$B/api/analytics/overview?scope=all" -H "Authorization: Bearer $PTOK"
echo "== dashboard overview"; curl -s "$B/api/analytics/overview?days=7" -H "$A" | j "(d['summary'], d['games'][0]['title'], d['custom_events'], d['session_duration'], len(d['daily']))"
echo "== game analytics"; curl -s "$B/api/analytics/games/$ID?days=7" -H "$A" | j "(d['summary']['sessions'], d['summary']['win_rate'], d['summary']['completion_rate'])"
echo "== unpublish -> events ignored, play hidden"; curl -s -XPOST $B/api/games/$ID/unpublish -H "$A" | j "d['published']"; curl -s -o /dev/null -w "%{http_code}\n" $B/g/$ID
