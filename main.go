package main

import (
	"github.com/gin-gonic/gin"
	"backend/routes"
	"backend/configs"
	"log"
)

func main() {
	// Initialize configuration
	if err := configs.InitConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup router using the routes package
	r := routes.SetupRouter()
	
	// Get server config
	config := configs.GetConfig()
	
	// Listen and serve on the configured address
	if err := r.Run(config.GetServerAddress()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}