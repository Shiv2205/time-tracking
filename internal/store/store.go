package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"time-tracking/internal/model"
)

var ErrConflict = errors.New("entry changed in another window; refresh before continuing")

type Store struct {
	db  *sql.DB
	dir string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "app.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, dir: dir}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS drafts (id TEXT PRIMARY KEY, date TEXT NOT NULL, revision INTEGER NOT NULL, body TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS drafts_date ON drafts(date);
CREATE TABLE IF NOT EXISTS logs (id TEXT PRIMARY KEY, date TEXT NOT NULL, body TEXT NOT NULL);
PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	entries, err := s.AllDrafts()
	if err != nil {
		db.Close()
		return nil, err
	}
	for _, d := range entries {
		if d.Status == "submitting" {
			rev := d.Revision
			d.Status = "uncertain"
			d.Error = "Submission was interrupted. Check Zoho before retrying."
			if err = s.SaveDraft(&d, rev); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Get(key string, out any) error {
	var raw string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), out)
}
func (s *Store) Set(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, string(raw))
	return err
}
func (s *Store) Draft(id string) (model.Draft, error) {
	var d model.Draft
	var raw string
	err := s.db.QueryRow("SELECT body FROM drafts WHERE id=?", id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &d)
	}
	return d, err
}
func (s *Store) AllDrafts() ([]model.Draft, error) {
	return s.drafts("SELECT body FROM drafts ORDER BY date,id")
}
func (s *Store) Drafts(start, end string) ([]model.Draft, error) {
	return s.drafts("SELECT body FROM drafts WHERE date BETWEEN ? AND ? ORDER BY date,id", start, end)
}
func (s *Store) drafts(query string, args ...any) ([]model.Draft, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.Draft{}
	for rows.Next() {
		var raw string
		var d model.Draft
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &d); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (s *Store) SaveDraft(d *model.Draft, expected int) error {
	d.Revision = expected + 1
	d.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if expected == 0 {
		_, err = s.db.Exec("INSERT INTO drafts(id,date,revision,body) VALUES(?,?,?,?)", d.ID, d.Date, d.Revision, string(raw))
		return err
	}
	r, err := s.db.Exec("UPDATE drafts SET date=?,revision=?,body=? WHERE id=? AND revision=?", d.Date, d.Revision, string(raw), d.ID, expected)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (s *Store) DeleteDraft(id string, revision int) error {
	r, err := s.db.Exec("DELETE FROM drafts WHERE id=? AND revision=?", id, revision)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (s *Store) ReplaceLogs(start, end string, logs []model.Log) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM logs WHERE date BETWEEN ? AND ?", start, end); err != nil {
		return err
	}
	for _, l := range logs {
		raw, e := json.Marshal(l)
		if e != nil {
			return e
		}
		if _, err = tx.Exec("INSERT OR REPLACE INTO logs(id,date,body) VALUES(?,?,?)", l.ID, l.Date, string(raw)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Logs(start, end string) ([]model.Log, error) {
	rows, err := s.db.Query("SELECT body FROM logs WHERE date BETWEEN ? AND ? ORDER BY date,id", start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.Log{}
	for rows.Next() {
		var raw string
		var l model.Log
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &l); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
func (s *Store) Backup() ([]byte, error) {
	f, err := os.CreateTemp(s.dir, "backup-*.db")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	if err = os.Remove(path); err != nil {
		return nil, err
	}
	defer os.Remove(path)
	_, err = s.db.Exec("VACUUM INTO '" + strings.ReplaceAll(path, "'", "''") + "'")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
