package postgres

import (
	"context"
	"errors"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *repository) CreateGame(ctx context.Context, ownerID, prompt string) (string, error) {
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx, `INSERT INTO games(id,owner_id,prompt,status) VALUES($1,$2,$3,'planning')`, id, ownerID, prompt)
	return id, err
}

func (r *repository) AddVersion(ctx context.Context, in domain.VersionInput) (int, error) {

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var v int
	err = tx.QueryRow(ctx,
		`UPDATE games SET html=$2, version=version+1,
		   title    = CASE WHEN $3<>'' THEN $3 ELSE title END,
		   genre    = CASE WHEN $4<>'' THEN $4 ELSE genre END,
		   audience = CASE WHEN $5<>'' THEN $5 ELSE audience END,
		   updated_at=now() WHERE id=$1 RETURNING version`,
		in.GameID, in.HTML, in.Title, in.Genre, in.Audience).Scan(&v)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, coreErrs.ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	instruction := in.Instruction
	if instruction == "" {
		instruction = in.UserText
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO game_versions(game_id,version,html,instruction) VALUES($1,$2,$3,$4)`,
		in.GameID, v, in.HTML, instruction)
	if err != nil {
		return 0, err
	}

	if in.UserText != "" {
		_, err = tx.Exec(ctx,
			`INSERT INTO messages(game_id,role,content) VALUES($1,'user',$2)`, in.GameID, in.UserText)
		if err != nil {
			return 0, err
		}
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO messages(game_id,role,content,version) VALUES($1,'assistant',$2,$3)`,
		in.GameID, in.Reply, v)
	if err != nil {
		return 0, err
	}

	return v, tx.Commit(ctx)
}
