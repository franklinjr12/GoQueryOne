package main

import (
	"log"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

func main() {

	a := app.New()
	w := a.NewWindow("Hello World")

	w.SetContent(widget.NewLabel("Hello World!"))
	w.ShowAndRun()

	// Configuration constants - set these as needed
	const (
		dsn            = "your_odbc_dsn_here" // Set your ODBC DSN here
		query          = "SELECT 1"           // Set your SQL query here
		configFile     = ""                   // Set config file path if using one, or leave empty
		testConnection = false                // Set to true to test connection only
	)

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("GoQueryOne - ODBC Query Tool")

	// Load configuration if provided
	var cfg *config.Config
	var err error
	if configFile != "" {
		cfg, err = config.LoadConfig(configFile)
		if err != nil {
			log.Printf("Warning: failed to load config file: %v", err)
			cfg = config.DefaultConfig()
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Use constant DSN if provided, otherwise use config
	if dsn != "" {
		cfg.Database.DSN = dsn
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Check if DSN is provided
	if cfg.Database.DSN == "" {
		log.Fatal("DSN is required. Set the dsn constant in main.go or provide it in config file.")
	}

	// Create and establish connection
	conn := odbc.NewConnection(cfg.Database.DSN)

	log.Printf("Connecting to database: %s", cfg.Database.DSN)
	if err := conn.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := conn.Disconnect(); err != nil {
			log.Printf("Warning: failed to disconnect: %v", err)
		}
	}()

	// Test connection if requested
	if testConnection {
		log.Println("Testing connection...")
		if err := conn.TestConnection(); err != nil {
			log.Fatalf("Connection test failed: %v", err)
		}
		log.Println("Connection test successful!")
		return
	}

	// Check if query is provided
	if query == "" {
		log.Fatal("Query is required. Set the query constant in main.go.")
	}

	// Execute query
	log.Printf("Executing query: %s", query)
	result, err := conn.ExecuteQuery(query)
	if err != nil {
		log.Fatalf("Query execution failed: %v", err)
	}

	// Display results
	odbc.DisplayResult(result)

	log.Println("Query execution completed successfully")
}
