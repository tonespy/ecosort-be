package main

import (
	"log"

	"github.com/tonespy/ecosort_be/config"
	"github.com/tonespy/ecosort_be/internal/server"
	"github.com/tonespy/ecosort_be/pkg/logger"
)

func main() {
	app_config, err := config.PrepareConfig()
	if err != nil {
		panic(err)
	}

	// Download latest model
	err = config.DownloadModel(*app_config)
	if err != nil {
		panic(err)
	}

	// Initialize logger
	appLogger := logger.NewLogger()

	server := server.Server{
		Logger: appLogger,
		Config: app_config,
	}

	router := server.NewRouter()
	err = router.Run(app_config.Port)
	if err != nil {
		log.Fatal(err)
	}
}
