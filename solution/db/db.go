package database

import (
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

func MigrateDB(db *sqlx.DB) {
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(db.DB, "/scripts"); err != nil {
		panic(err)
	}
}
