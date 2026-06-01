package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %v", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.initTables(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS processed_sms (
			id TEXT PRIMARY KEY,
			processed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS processed_calls (
			id TEXT PRIMARY KEY,
			processed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) IsSMSProcessed(id string) (bool, error) {
	var exists bool
	err := db.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM processed_sms WHERE id = ?)", id).Scan(&exists)
	return exists, err
}

func (db *DB) MarkSMSProcessed(id string) error {
	_, err := db.conn.Exec("INSERT OR IGNORE INTO processed_sms (id) VALUES (?)", id)
	return err
}

func (db *DB) IsCallProcessed(id string) (bool, error) {
	var exists bool
	err := db.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM processed_calls WHERE id = ?)", id).Scan(&exists)
	return exists, err
}

func (db *DB) MarkCallProcessed(id string) error {
	_, err := db.conn.Exec("INSERT OR IGNORE INTO processed_calls (id) VALUES (?)", id)
	return err
}
