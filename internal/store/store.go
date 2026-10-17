package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	_ "github.com/lib/pq"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Store struct{ DB *sql.DB }

// Open подключается к Postgres (с ретраями — удобно для docker-compose) и накатывает миграции.
func Open(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = db.PingContext(ctx); lastErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("postgres недоступен: %w", lastErr)
	}
	return &Store{DB: db}, nil
}

func NewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IsUUID(s string) bool { return uuidRe.MatchString(s) }

func randomKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- users ----------

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser возвращает пользователя и access_key — «пароль», который показывается один раз.
func (s *Store) CreateUser(ctx context.Context, name, role string) (User, string, error) {
	u := User{ID: NewUUID(), Name: name, Role: role}
	key := randomKey()
	err := s.DB.QueryRowContext(ctx,
		`INSERT INTO users(id,name,role,access_key) VALUES($1,$2,$3,$4) RETURNING created_at`,
		u.ID, name, role, key).Scan(&u.CreatedAt)
	return u, key, err
}

func (s *Store) UserByAccessKey(ctx context.Context, key string) (User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx,
		`SELECT id,name,role,created_at FROM users WHERE access_key=$1`, key).
		Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx,
		`SELECT id,name,role,created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// ---------- games ----------

type Game struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	OwnerName string    `json:"owner_name"`
	Title     string    `json:"title"`
	Genre     string    `json:"genre"`
	Audience  string    `json:"audience"`
	Prompt    string    `json:"prompt"`
	HTML      string    `json:"html,omitempty"`
	Version   int       `json:"version"`
	Published bool      `json:"published"`
	Plays     int64     `json:"plays"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const gameCols = `g.id,g.owner_id,u.name,g.title,g.genre,g.audience,g.prompt,g.version,g.published,g.plays,g.created_at,g.updated_at`

func scanGame(sc interface{ Scan(...any) error }, withHTML bool) (Game, error) {
	var g Game
	dest := []any{&g.ID, &g.OwnerID, &g.OwnerName, &g.Title, &g.Genre, &g.Audience, &g.Prompt,
		&g.Version, &g.Published, &g.Plays, &g.CreatedAt, &g.UpdatedAt}
	if withHTML {
		dest = append(dest, &g.HTML)
	}
	return g, sc.Scan(dest...)
}

func (s *Store) CreateGame(ctx context.Context, ownerID, prompt string) (string, error) {
	id := NewUUID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO games(id,owner_id,prompt) VALUES($1,$2,$3)`, id, ownerID, prompt)
	return id, err
}

func (s *Store) GetGame(ctx context.Context, id string) (Game, error) {
	g, err := scanGame(s.DB.QueryRowContext(ctx,
		`SELECT `+gameCols+`,g.html FROM games g JOIN users u ON u.id=g.owner_id WHERE g.id=$1`, id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

func (s *Store) ListOwnerGames(ctx context.Context, ownerID string, limit, offset int) ([]Game, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.owner_id=$1 AND g.version>0 ORDER BY g.updated_at DESC LIMIT $2 OFFSET $3`, ownerID, limit, offset)
	return collectGames(rows, err)
}

// ListPublished — галерея. sort: "popular" | "new". q — поиск по названию/жанру.
func (s *Store) ListPublished(ctx context.Context, sort, q string, limit, offset int) ([]Game, error) {
	order := "g.created_at DESC"
	if sort == "popular" {
		order = "g.plays DESC, g.created_at DESC"
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.published AND ($1='' OR g.title ILIKE '%'||$1||'%' OR g.genre ILIKE '%'||$1||'%')
		 ORDER BY `+order+` LIMIT $2 OFFSET $3`, q, limit, offset)
	return collectGames(rows, err)
}

func collectGames(rows *sql.Rows, err error) ([]Game, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Game{}
	for rows.Next() {
		g, err := scanGame(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type VersionInput struct {
	GameID, UserText, Reply, HTML, Title, Genre, Audience string
}

// AddVersion атомарно сохраняет новую версию игры и пару сообщений чата.
func (s *Store) AddVersion(ctx context.Context, in VersionInput) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var v int
	err = tx.QueryRowContext(ctx,
		`UPDATE games SET html=$2, version=version+1,
		   title    = CASE WHEN $3<>'' THEN $3 ELSE title END,
		   genre    = CASE WHEN $4<>'' THEN $4 ELSE genre END,
		   audience = CASE WHEN $5<>'' THEN $5 ELSE audience END,
		   updated_at=now() WHERE id=$1 RETURNING version`,
		in.GameID, in.HTML, in.Title, in.Genre, in.Audience).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO game_versions(game_id,version,html,instruction) VALUES($1,$2,$3,$4)`,
		in.GameID, v, in.HTML, in.UserText); err != nil {
		return 0, err
	}
	if in.UserText != "" {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO messages(game_id,role,content) VALUES($1,'user',$2)`, in.GameID, in.UserText); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO messages(game_id,role,content,version) VALUES($1,'assistant',$2,$3)`,
		in.GameID, in.Reply, v); err != nil {
		return 0, err
	}
	return v, tx.Commit()
}

type Message struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Version   *int      `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Messages(ctx context.Context, gameID string) ([]Message, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id,role,content,version,created_at FROM messages WHERE game_id=$1 ORDER BY id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var v sql.NullInt32
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &v, &m.CreatedAt); err != nil {
			return nil, err
		}
		if v.Valid {
			n := int(v.Int32)
			m.Version = &n
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type VersionInfo struct {
	Version     int       `json:"version"`
	Instruction string    `json:"instruction"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) Versions(ctx context.Context, gameID string) ([]VersionInfo, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT version,instruction,created_at FROM game_versions WHERE game_id=$1 ORDER BY version DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionInfo{}
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Instruction, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) VersionHTML(ctx context.Context, gameID string, v int) (string, error) {
	var h string
	err := s.DB.QueryRowContext(ctx,
		`SELECT html FROM game_versions WHERE game_id=$1 AND version=$2`, gameID, v).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return h, err
}

func (s *Store) SetPublished(ctx context.Context, id string, published bool) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE games SET published=$2, updated_at=now() WHERE id=$1`, id, published)
	return err
}

func (s *Store) SetTitle(ctx context.Context, id, title string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE games SET title=$2, updated_at=now() WHERE id=$1`, id, title)
	return err
}

func (s *Store) DeleteGame(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM games WHERE id=$1`, id)
	return err
}

func (s *Store) IsPublished(ctx context.Context, id string) (bool, error) {
	var p bool
	err := s.DB.QueryRowContext(ctx, `SELECT published FROM games WHERE id=$1`, id).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return p, err
}
