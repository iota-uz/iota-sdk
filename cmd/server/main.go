package main

import (
	"github.com/iota-agency/iota-erp/pkg/server"
	"github.com/iota-agency/iota-erp/pkg/utils"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func main() {
	utils.LoadEnv()
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := gorm.Open(postgres.Open(utils.DbOpts()), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	httpServer := server.New(db)
	httpServer.Start()

	//telegramServer := tgServer.New(db)
	//wg := sync.WaitGroup{}
	//wg.Add(2)
	//go func() {
	//	httpServer.Start()
	//	wg.Done()
	//}()
	//go func() {
	//	telegramServer.Start()
	//	wg.Done()
	//}()
	//wg.Wait()
}
