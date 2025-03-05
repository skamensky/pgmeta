package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionConfig(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test config file
	configPath := filepath.Join(tmpDir, "config.json")
	cfg := &Config{
		configPath: configPath,
		Connections: []Connection{
			{
				Name:      "test1",
				URL:       "host=localhost dbname=test1 user=postgres",
				IsDefault: true,
			},
		},
	}

	// Save the config
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test adding a connection
	if err := cfg.AddConnection("test2", "postgres://postgres:pass@localhost/test2", false); err != nil {
		t.Fatalf("Failed to add connection: %v", err)
	}

	// Verify the connection was added
	if len(cfg.Connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(cfg.Connections))
	}

	// Test getting default connection
	defaultConn := cfg.GetDefaultConnection()
	if defaultConn == nil {
		t.Fatalf("Default connection is nil")
	}
	if defaultConn.Name != "test1" {
		t.Errorf("Expected default connection to be test1, got %s", defaultConn.Name)
	}

	// Test setting a new default connection
	if err := cfg.SetDefaultConnection("test2"); err != nil {
		t.Fatalf("Failed to set default connection: %v", err)
	}

	// Verify the new default
	defaultConn = cfg.GetDefaultConnection()
	if defaultConn == nil {
		t.Fatalf("Default connection is nil after change")
	}
	if defaultConn.Name != "test2" {
		t.Errorf("Expected default connection to be test2, got %s", defaultConn.Name)
	}

	// Test getting a connection by name
	conn := cfg.GetConnection("test1")
	if conn == nil {
		t.Fatalf("Failed to get connection by name")
	}
	if conn.Name != "test1" {
		t.Errorf("Expected connection name to be test1, got %s", conn.Name)
	}

	// Test deleting a connection
	if err := cfg.DeleteConnection("test1"); err != nil {
		t.Fatalf("Failed to delete connection: %v", err)
	}

	// Verify the connection was deleted
	if len(cfg.Connections) != 1 {
		t.Errorf("Expected 1 connection after deletion, got %d", len(cfg.Connections))
	}
	if cfg.GetConnection("test1") != nil {
		t.Errorf("Connection test1 still exists after deletion")
	}
}

func TestConnectionConfigErrors(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test config
	configPath := filepath.Join(tmpDir, "config.json")
	cfg := &Config{
		configPath: configPath,
	}

	// Test adding connection with empty name
	err = cfg.AddConnection("", "host=localhost", false)
	if err == nil {
		t.Error("Expected error when adding connection with empty name, got nil")
	}

	// Add a valid connection
	if err := cfg.AddConnection("test", "host=localhost", true); err != nil {
		t.Fatalf("Failed to add valid connection: %v", err)
	}

	// Test adding duplicate connection
	err = cfg.AddConnection("test", "host=localhost", false)
	if err == nil {
		t.Error("Expected error when adding duplicate connection, got nil")
	}

	// Test setting non-existent connection as default
	err = cfg.SetDefaultConnection("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent connection as default, got nil")
	}

	// Test deleting non-existent connection
	err = cfg.DeleteConnection("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent connection, got nil")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "pgmeta-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME environment variable to the temp dir for this test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// First, test loading with no config file (should create a new one)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if len(cfg.Connections) != 0 {
		t.Errorf("Expected empty connections list, got %d connections", len(cfg.Connections))
	}

	// Save a test config file
	configDir := filepath.Join(tmpDir, ".pgmeta")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	testConfig := struct {
		Connections []Connection `json:"connections"`
	}{
		Connections: []Connection{
			{
				Name:      "testconn",
				URL:       "host=localhost",
				IsDefault: true,
			},
		},
	}

	configData, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Now load the config again and verify it loads correctly
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load existing config: %v", err)
	}
	if len(cfg.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(cfg.Connections))
	}
	if cfg.Connections[0].Name != "testconn" {
		t.Errorf("Expected connection name to be testconn, got %s", cfg.Connections[0].Name)
	}
}
