package main

import (
	"fmt"
	"github.com/iota-agency/iota-erp/pkg/migration"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/kulado/sqlxmigrate"
	_ "github.com/lib/pq"
)

func loadMigrations() ([]*sqlxmigrate.Migration, error) {
	//var migrations []*sqlxmigrate.Migration
	files, err := os.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	migrations := make([]*sqlxmigrate.Migration, len(files))
	for i, file := range files {
		m, err := migration.LoadMigration("migrations/" + file.Name())
		if err != nil {
			return nil, fmt.Errorf("could not parse migration %s: %v", file.Name(), err)
		}
		migrations[i] = m
	}
	return migrations, nil
}

func migrateUp(to string) {
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := sqlx.Connect("postgres", utils.DbOpts())
	if err != nil {
		log.Fatalf("main : Register DB : %v", err)
	}
	defer db.Close()
	migrations, err := loadMigrations()
	if err != nil {
		log.Fatalf("Could not load migrations: %v", err)
	}
	m := sqlxmigrate.New(db, sqlxmigrate.DefaultOptions, migrations)
	if to != "" {
		err = m.MigrateTo(to)
	} else {
		err = m.Migrate()
	}
	if err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	log.Printf("Migrations ran successfully")
}

func migrateDown(to string) {
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := sqlx.Connect("postgres", utils.DbOpts())
	if err != nil {
		log.Fatalf("main : Register DB : %v", err)
	}
	defer db.Close()
	migrations, err := loadMigrations()
	if err != nil {
		log.Fatalf("Could not load migrations: %v", err)
	}
	m := sqlxmigrate.New(db, sqlxmigrate.DefaultOptions, migrations)
	if to != "" {
		err = m.RollbackTo(to)
	} else {
		err = m.RollbackLast()
	}
	if err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	log.Printf("Migrations ran successfully")
}

func main() {
	utils.LoadEnv()
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <command>", os.Args[0])
	}
	cmd := os.Args[1]
	var to string
	if len(os.Args) > 2 {
		to = os.Args[2]
	}
	if cmd == "up" {
		migrateUp(to)
	} else if cmd == "down" {
		migrateDown(to)
	} else {
		log.Fatalf("Unknown command %s", cmd)
	}
}
