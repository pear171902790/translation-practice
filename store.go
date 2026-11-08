package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Sentence is one saved Chinese sentence and its reference English translation.
type Sentence struct {
	ID        int64
	Chinese   string
	English   string
	CreatedAt time.Time
}

// CreatedLabel is the save time in the machine's local timezone.
func (s Sentence) CreatedLabel() string {
	if s.CreatedAt.IsZero() {
		return ""
	}
	return s.CreatedAt.Local().Format("2006-01-02 15:04")
}

// Store persists sentences in a SQLite file. The driver is pure Go (no CGO).
type Store struct {
	db *sql.DB
}

// Open creates the parent directory and the sentences table when missing.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// One connection keeps SQLite's write lock simple for this single-user app.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sentences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chinese TEXT NOT NULL,
			english TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Create(ctx context.Context, chinese, english string) (Sentence, error) {
	created := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO sentences (chinese, english, created_at) VALUES (?, ?, ?)`,
		chinese, english, created.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Sentence{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Sentence{}, err
	}
	return Sentence{ID: id, Chinese: chinese, English: english, CreatedAt: created}, nil
}

func (s *Store) List(ctx context.Context) ([]Sentence, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chinese, english, created_at FROM sentences ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Sentence
	for rows.Next() {
		sentence, err := scanSentence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sentence)
	}
	return out, rows.Err()
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sentences`).Scan(&n)
	return n, err
}

func (s *Store) Get(ctx context.Context, id int64) (Sentence, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, chinese, english, created_at FROM sentences WHERE id = ?`, id)
	return scanSentence(row)
}

func (s *Store) Random(ctx context.Context) (Sentence, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, chinese, english, created_at FROM sentences ORDER BY RANDOM() LIMIT 1`)
	return scanSentence(row)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sentences WHERE id = ?`, id)
	return err
}

type sample struct {
	chinese string
	english string
}

var sampleSentences = []sample{
	{"今天天气很好。", "The weather is nice today."},
	{"我每天早上喝咖啡。", "I drink coffee every morning."},
	{"请问去火车站怎么走？", "Excuse me, how do I get to the train station?"},
	{"这本书很有意思。", "This book is very interesting."},
}

// SeedSamples inserts the built-in examples only when the table is empty.
// It returns how many rows were inserted.
func (s *Store) SeedSamples(ctx context.Context) (int, error) {
	n, err := s.Count(ctx)
	if err != nil || n > 0 {
		return 0, err
	}
	for _, item := range sampleSentences {
		if _, err := s.Create(ctx, item.chinese, item.english); err != nil {
			return 0, err
		}
	}
	return len(sampleSentences), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSentence(row scanner) (Sentence, error) {
	var (
		sentence Sentence
		created  string
	)
	if err := row.Scan(&sentence.ID, &sentence.Chinese, &sentence.English, &created); err != nil {
		return Sentence{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, created)
		if err != nil {
			return Sentence{}, fmt.Errorf("parse created_at: %w", err)
		}
	}
	sentence.CreatedAt = parsed
	return sentence, nil
}
