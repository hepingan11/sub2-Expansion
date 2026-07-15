package app

import (
	"context"
	"log"
)

func Run() {
	loadDotEnv()

	cfg := loadConfig()
	db, err := openDatabase(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	app := &App{db: db, cfg: cfg}
	if err := app.migrate(); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	app.startSub2APITokenRefresher(context.Background())
	app.startSub2APIGroupRateMonitor(context.Background())
	app.startTelegramBot(context.Background())

	router := app.router()
	log.Printf("Go backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
