package odbc

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "github.com/alexbrainman/odbc"
)

// NewConnection creates a new ODBC connection instance
func NewConnection(dsn string) *Connection {
	return &Connection{
		DSN:       dsn,
		connected: false,
	}
}

// Connect establishes a connection to the ODBC data source
func (c *Connection) Connect() error {
	if c.connected {
		log.Printf("Error: already connected to %s", c.DSN)
		return errors.New("already connected to " + c.DSN)
	}

	log.Printf("Connecting to ODBC data source: %s", c.DSN)

	// Connect to the ODBC data source using database/sql with ODBC driver
	conn, err := sql.Open("odbc", c.DSN)
	if err != nil {
		log.Printf("Error: failed to connect to %s: %v", c.DSN, err)
		return errors.New("failed to connect to " + c.DSN + ": " + err.Error())
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		log.Printf("Error: failed to ping database %s: %v", c.DSN, err)
		return errors.New("failed to ping database " + c.DSN + ": " + err.Error())
	}

	c.Handle = conn
	c.connected = true
	c.ConnectTime = time.Now()

	log.Printf("Successfully connected to %s at %s", c.DSN, c.ConnectTime.Format(time.RFC3339))
	return nil
}

// Disconnect closes the ODBC connection
func (c *Connection) Disconnect() error {
	if !c.connected {
		log.Printf("Error: not connected to %s", c.DSN)
		return errors.New("not connected to " + c.DSN)
	}

	log.Printf("Disconnecting from %s", c.DSN)

	if c.Handle != nil {
		err := c.Handle.Close()
		if err != nil {
			log.Printf("Error: failed to close connection to %s: %v", c.DSN, err)
			return errors.New("failed to close connection to " + c.DSN + ": " + err.Error())
		}
	}

	c.Handle = nil
	c.connected = false

	log.Printf("Successfully disconnected from %s", c.DSN)
	return nil
}

// IsConnected returns the current connection status
func (c *Connection) IsConnected() bool {
	return c.connected
}

// TestConnection validates the current connection
func (c *Connection) TestConnection() error {
	if !c.connected {
		log.Printf("Error: not connected to %s", c.DSN)
		return errors.New("not connected to " + c.DSN)
	}

	// Execute a simple test query
	_, err := c.ExecuteQuery("SELECT 1")
	if err != nil {
		log.Printf("Error: connection test failed: %v", err)
		return errors.New("connection test failed: " + err.Error())
	}

	log.Printf("Connection test successful for %s", c.DSN)
	return nil
}

// GetConnectionInfo returns information about the current connection
func (c *Connection) GetConnectionInfo() map[string]interface{} {
	info := map[string]interface{}{
		"dsn":          c.DSN,
		"is_connected": c.connected,
		"connect_time": c.ConnectTime,
		"last_query":   c.LastQueryTime,
	}

	if c.connected {
		info["connection_duration"] = time.Since(c.ConnectTime)
	}

	return info
}
