//go:build integration

// Запуск: TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/features/experiments/...
// База должна быть с применёнными миграциями; тест пишет в неё свои строки и не чистит их.
package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beeline-games/internal/core/domain"
	gamesRepo "beeline-games/internal/features/games/repository/postgres"
)

func TestExperimentLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo, games := NewRepository(pool), gamesRepo.NewRepository(pool)
	owner := uuid.NewString()
	mustExec(t, pool, `INSERT INTO users(id,name,role,access_key) VALUES($1,'автор','manager',$2)`, owner, owner)

	id, err := repo.Create(ctx, owner, "змейка", "змейка для бабушки")
	if err != nil {
		t.Fatal(err)
	}
	variant := func(label, build string) string {
		gid, err := games.CreateVariant(ctx, domain.VariantInput{
			OwnerID: owner, ExperimentID: id, Label: label, Direction: "направление " + label, Prompt: "змейка",
		})
		if err != nil {
			t.Fatal(err)
		}
		if build == "" {
			mustExec(t, pool, `UPDATE games SET status='failed' WHERE id=$1`, gid)
			return gid
		}
		if _, err := games.AddVersion(ctx, domain.VersionInput{GameID: gid, Reply: "готово", HTML: "<html></html>"}); err != nil {
			t.Fatal(err)
		}
		mustExec(t, pool, `UPDATE games SET status='ready' WHERE id=$1`, gid)
		mustExec(t, pool, `INSERT INTO generations(game_id,kind,status,cost_usd) VALUES($1,'plan','ready',0.01),($1,'build',$2,0.2)`, gid, build)
		// неудачная правка после сборки не должна лишать вариант допуска
		mustExec(t, pool, `INSERT INTO generations(game_id,kind,status) VALUES($1,'edit','failed')`, gid)
		return gid
	}
	a, b, c, d := variant("A", "ready"), variant("B", "ready"), variant("C", "ready_with_issues"), variant("D", "")

	vs, err := repo.Variants(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 4 || vs[0].GameID != a || vs[3].GameID != d {
		t.Fatalf("варианты не по меткам: %+v", vs)
	}
	eligible := map[string]bool{}
	for _, v := range vs {
		eligible[v.GameID] = v.Eligible()
	}
	if !eligible[a] || !eligible[b] || eligible[c] || eligible[d] {
		t.Errorf("допуск вариантов: %v", eligible)
	}
	if vs[0].CostUSD < 0.209 || vs[0].Direction != "направление A" {
		t.Errorf("стоимость или направление варианта потеряны: %+v", vs[0])
	}

	x, err := repo.Get(ctx, id)
	if err != nil || x.Variants != 4 || x.Ready != 3 || x.Status != domain.ExperimentDraft {
		t.Fatalf("сводка эксперимента: %+v, %v", x, err)
	}
	if list, err := repo.ListOwner(ctx, owner, 10); err != nil || len(list) != 1 {
		t.Fatalf("список экспериментов: %+v, %v", list, err)
	}

	if rt, err := repo.Routing(ctx, id); err != nil || len(rt.Live) != 0 || rt.Status != domain.ExperimentDraft {
		t.Fatalf("маршрутизация черновика: %+v, %v", rt, err)
	}
	for _, gid := range []string{a, b} {
		if err := games.SetPublished(ctx, gid, true); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := repo.Launch(ctx, id); err != nil || !ok {
		t.Fatalf("запуск: %v, %v", ok, err)
	}
	if ok, _ := repo.Launch(ctx, id); ok {
		t.Error("эксперимент запущен повторно")
	}
	rt, err := repo.Routing(ctx, id)
	if err != nil || rt.Status != domain.ExperimentRunning || len(rt.Live) != 2 || rt.Live[0] != a || rt.Live[1] != b {
		t.Fatalf("маршрутизация запущенного: %+v, %v", rt, err)
	}

	// A: два показа, оба загрузились, один доигран до порога; B: показ без загрузки.
	// Прямой заход на A (сессия без показа) в сравнение попасть не должен.
	mustExec(t, pool, `INSERT INTO events(game_id,session_id,player_id,type,duration_ms) VALUES
		($1,'s1','p1','exposure',NULL),($1,'s1','p1','session_start',NULL),($1,'s1','p1','engaged',NULL),
		($1,'s1','p1','session_end',45000),
		($1,'s2','p2','exposure',NULL),($1,'s2','p2','session_start',NULL),($1,'s2','p2','js_error',NULL),
		($1,'s2','p2','session_end',5000),
		($1,'direct','p9','session_start',NULL),($1,'direct','p9','session_end',90000),
		($2,'s3','p3','exposure',NULL)`, a, b)

	stats, err := repo.VariantStats(ctx, id, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 4 {
		t.Fatalf("статистика не по всем вариантам: %+v", stats)
	}
	sa, sb := stats[0], stats[1]
	if sa.GameID != a || sa.Exposures != 2 || sa.Sessions != 2 || sa.Engaged != 1 || sa.Retained != 1 ||
		sa.ErrorSessions != 1 || sa.Players != 2 {
		t.Errorf("воронка A: %+v", sa)
	}
	if sa.AvgSessionSec != 25 {
		t.Errorf("средняя сессия A = %v, ожидалось 25 (прямой заход не учитывается)", sa.AvgSessionSec)
	}
	if sb.GameID != b || sb.Exposures != 1 || sb.Sessions != 0 || sb.LoadRate != 0 {
		t.Errorf("воронка B: %+v", sb)
	}
	if stats[3].Exposures != 0 {
		t.Errorf("у неопубликованного варианта есть показы: %+v", stats[3])
	}

	if gallery, err := games.ListPublished(ctx, domain.GameQuery{Limit: 100}); err != nil || containsGame(gallery, a) {
		t.Fatalf("вариант до выбора победителя попал на витрину: %v", err)
	}
	arms, err := repo.Arms(ctx, id, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	byGame := map[string]domain.ArmStats{}
	for _, a := range arms {
		byGame[a.GameID] = a
	}
	if byGame[a].Trials != 2 || byGame[a].Successes != 1 || byGame[b].Trials != 1 || byGame[b].Successes != 0 {
		t.Errorf("данные бандита: %+v", arms)
	}
	if early, _ := repo.Arms(ctx, id, time.Now().Add(-time.Hour)); len(early) != 0 {
		t.Errorf("неустоявшиеся показы попали к бандиту: %+v", early)
	}

	if got, err := repo.Assignment(ctx, id, "p1"); err != nil || got != "" {
		t.Fatalf("закрепление нового игрока: %q, %v", got, err)
	}
	for _, gid := range []string{a, b} {
		if err := repo.Assign(ctx, id, "p1", gid); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := repo.Assignment(ctx, id, "p1"); err != nil || got != b {
		t.Fatalf("переназначение игрока: %q, %v", got, err)
	}

	if ok, err := repo.Finish(ctx, id, b, domain.WinnerAuto); err != nil || !ok {
		t.Fatalf("выбор победителя: %v, %v", ok, err)
	}
	if ok, _ := repo.Finish(ctx, id, a, domain.WinnerAuto); ok {
		t.Error("бандит перебил уже выбранного победителя")
	}
	gallery, err := games.ListPublished(ctx, domain.GameQuery{Limit: 100})
	if err != nil || !containsGame(gallery, b) || containsGame(gallery, a) {
		t.Fatalf("на витрине должен быть только победитель: %v", err)
	}
	if rt, _ := repo.Routing(ctx, id); rt.WinnerID != b || rt.Status != domain.ExperimentFinished {
		t.Errorf("маршрутизация завершённого: %+v", rt)
	}
	if x, _ := repo.Get(ctx, id); x.WinnerSource != domain.WinnerAuto {
		t.Errorf("источник победы = %q, ожидался auto", x.WinnerSource)
	}
	if ok, err := repo.Finish(ctx, id, a, domain.WinnerManual); err != nil || !ok {
		t.Fatalf("автор не смог поменять победителя: %v, %v", ok, err)
	}
	if g, err := games.GetGame(ctx, b); err != nil || g.ExperimentID != id || g.VariantLabel != "B" {
		t.Errorf("игра не знает свой эксперимент: %+v, %v", g, err)
	}
}

func containsGame(gs []domain.Game, id string) bool {
	for _, g := range gs {
		if g.ID == id {
			return true
		}
	}
	return false
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
