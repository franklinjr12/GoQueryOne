package main

import (
	"log"

	"github.com/franklinjr12/GoQueryOne/internal/config"
	"github.com/franklinjr12/GoQueryOne/internal/odbc"
)

func main() {
	cfg := config.DefaultConfig()
	manager := odbc.NewManager()

	profile := config.ConnectionProfile{
		ID:   "example",
		Name: "Example Connection",
		Type: "dsn",
		DSN:  "your_dsn",
	}

	log.Printf("Default timeout: %d ms", cfg.Query.DefaultTimeoutMs)
	log.Printf("Profile string build test: %+v", profile)
	_ = manager
}
