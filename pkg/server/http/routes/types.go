package routes

import (
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
)

type Options struct {
	Db *sqlx.DB
}

type Route interface {
	Prefix() string
	Setup(router *mux.Router, opts *Options)
}
