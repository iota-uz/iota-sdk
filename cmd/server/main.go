package main

import (
	"github.com/iota-agency/iota-erp/pkg/server"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
)

func main() {
	utils.LoadEnv()
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := sqlx.Connect("postgres", utils.DbOpts())
	if err != nil {
		panic(err)
	}
	srv := server.New(db)
	srv.Start()
}
