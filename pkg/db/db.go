package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var (
	db *sql.DB
	tx *sql.Tx
)

func Commit() error {
	err := tx.Commit()
	if err != nil {
		return err
	}
	tx = nil
	return startTransaction()
}

func Connect(path string, readOnly bool) error {
	// if db != nil {
	// 	return -1
	// }

	var err error
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_time_format=sqlite", path)
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		log.Printf("db.Connect: %v", err)
		_ = db.Close()
		db = nil
		return err
	}

	db.SetMaxOpenConns(2) // Allow transaction + prepare operations
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)

	return startTransaction()
}

func Detach() int {
	// TODO?
	return -1
}

func Disconnect() error {
	if db == nil {
		return nil
	}
	if err := Commit(); err != nil { // DISCONNECT issues a COMMIT in Informix.
		return err
	}
	return nil
}

func Exec(query string, args ...any) (sql.Result, error) {
	return tx.Exec(query, args...)
}

func Exit() error {
	if err := db.Close(); err != nil {
		return err
	}
	tx = nil
	db = nil
	return nil
}

func Prepare(query string) (*sql.Stmt, error) {
	return db.Prepare(query)
}

func Rollback() error {
	if err := tx.Rollback(); err != nil {
		return err
	}
	tx = nil
	return startTransaction()
}

func startTransaction() error {
	var err error
	tx, err = db.Begin()
	return err
}

func Tx() *sql.Tx {
	return tx
}
