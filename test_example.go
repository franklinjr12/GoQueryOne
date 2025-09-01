package main

import (
	"log"

	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

func main() {
	// Example usage of GoQueryOne components

	// Create a default configuration
	cfg := config.DefaultConfig()
	cfg.Database.DSN = "your_odbc_dsn_here"

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		log.Printf("Configuration error: %v", err)
		return
	}

	log.Println("Configuration is valid")
	log.Printf("DSN: %s", cfg.Database.DSN)

	// Create a new connection
	conn := odbc.NewConnection(cfg.Database.DSN)

	// Note: In a real application, you would connect and execute queries
	// This is just a demonstration of the API structure
	log.Printf("Connection created for DSN: %s", conn.DSN)

	// Example of how to use the application:
	// 1. conn.Connect() - to establish connection
	// 2. conn.ExecuteQuery("SELECT * FROM your_table") - to execute queries
	// 3. conn.Disconnect() - to close connection
	//
	// Note: In the main application, these parameters are set as constants
	// in cmd/main.go instead of being passed as command line arguments.
}
