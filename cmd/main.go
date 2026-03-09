package main

import (
	"log"
	"strconv"

	"gotik/internal/config"
	"gotik/internal/db"
	apphttp "gotik/internal/http"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	sqlDB, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer func() {
		if err := db.CloseDB(sqlDB); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.AutoMigrate(sqlDB); err != nil {
		log.Fatalf("failed to auto migrate database: %v", err)
	}

	r := apphttp.SetRouter(sqlDB)
	log.Printf("server is running on port %d", cfg.Server.Port)
	if err := r.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
