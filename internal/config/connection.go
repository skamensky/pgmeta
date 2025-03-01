package config

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lib/pq"
	"github.com/palantir/stacktrace"
	"github.com/shkamensky/pgmeta/internal/log"
)

// Connection represents a database connection configuration
type Connection struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	IsDefault bool   `json:"is_default"`
}

// Config manages connection configurations
type Config struct {
	Connections []Connection `json:"connections"`
	configPath  string
}

// LoadConfig loads the configuration from disk
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to get home directory")
	}

	configDir := filepath.Join(homeDir, ".pgmeta")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, stacktrace.Propagate(err, "Failed to create config directory at %s", configDir)
	}

	configPath := filepath.Join(configDir, "config.json")
	cfg := &Config{configPath: configPath}
	log.Debug("Using config file: %s", configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Info("No config file found, creating a new one")
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to read config file at %s", configPath)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, stacktrace.Propagate(err, "Failed to parse config file: %v", err)
	}

	log.Debug("Loaded %d connections from config", len(cfg.Connections))
	return cfg, nil
}

// Save persists the configuration to disk
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return stacktrace.Propagate(err, "Failed to marshal config to JSON")
	}

	if err := os.WriteFile(c.configPath, data, 0644); err != nil {
		return stacktrace.Propagate(err, "Failed to write config to %s", c.configPath)
	}

	log.Debug("Config saved to %s", c.configPath)
	return nil
}

// AddConnection adds a new database connection
func (c *Config) AddConnection(name, url string, makeDefault bool) error {
	// Validate name
	if name == "" {
		return stacktrace.NewError("Connection name cannot be empty")
	}

	// Check for duplicate names
	if c.GetConnection(name) != nil {
		return stacktrace.NewError("Connection with name '%s' already exists", name)
	}

	// Normalize protocol
	url = strings.Replace(url, "postgresql://", "postgres://", 1)

	// If it's a URL format, parse and convert to connection string
	if strings.HasPrefix(url, "postgres://") {
		// URL encode special characters in password if needed
		if !strings.Contains(url, "%") { // Only if not already encoded
			parts := strings.Split(url, "@")
			if len(parts) == 2 {
				credentials := strings.Split(parts[0], ":")
				if len(credentials) == 3 { // protocol:user:pass
					password := credentials[2]
					encodedPassword := neturl.QueryEscape(password)
					if password != encodedPassword {
						url = credentials[0] + ":" + credentials[1] + ":" + encodedPassword + "@" + parts[1]
					}
				}
			}
		}

		log.Debug("Converting URL to connection string")
		// Convert to connection string
		connStr, err := pq.ParseURL(url)
		if err != nil {
			return stacktrace.Propagate(err, "Invalid connection URL: %s", url)
		}
		url = connStr
	}

	// Ensure required parameters
	params := make(map[string]string)
	for _, param := range strings.Split(url, " ") {
		if parts := strings.Split(param, "="); len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}

	// Force IPv4
	if host, ok := params["host"]; ok {
		params["hostaddr"] = host // Use IP address instead of hostname
	} else {
		params["host"] = "localhost"
	}
	
	// Ensure SSL mode is set
	if _, ok := params["sslmode"]; !ok {
		params["sslmode"] = "disable"
	}

	// Rebuild connection string
	var connParams []string
	for k, v := range params {
		// Skip logging sensitive parameters
		if k != "password" {
			log.Debug("Connection parameter %s=%s", k, v)
		}
		connParams = append(connParams, fmt.Sprintf("%s=%s", k, v))
	}
	url = strings.Join(connParams, " ")

	// If this is the first connection, make it default
	if len(c.Connections) == 0 {
		makeDefault = true
		log.Debug("First connection, automatically making it default")
	}

	// If making this default, unset other defaults
	if makeDefault {
		for i := range c.Connections {
			c.Connections[i].IsDefault = false
		}
	}

	c.Connections = append(c.Connections, Connection{
		Name:      name,
		URL:       url,
		IsDefault: makeDefault,
	})

	log.Info("Added connection '%s'%s", name, map[bool]string{true: " (default)", false: ""}[makeDefault])
	return c.Save()
}

// GetDefaultConnection returns the default connection
func (c *Config) GetDefaultConnection() *Connection {
	for _, conn := range c.Connections {
		if conn.IsDefault {
			return &conn
		}
	}
	
	// If there's no default but only one connection, use that one
	if len(c.Connections) == 1 {
		log.Debug("No default connection found, but only one connection exists - using it")
		return &c.Connections[0]
	}
	
	log.Debug("No default connection found")
	return nil
}

// DeleteConnection removes a connection by name
func (c *Config) DeleteConnection(name string) error {
	for i, conn := range c.Connections {
		if conn.Name == name {
			// If deleting default connection, set a new default
			if conn.IsDefault && len(c.Connections) > 1 {
				// Make the next connection default, or the previous if this is the last one
				nextIdx := (i + 1) % len(c.Connections)
				c.Connections[nextIdx].IsDefault = true
				log.Info("Setting '%s' as the new default connection", c.Connections[nextIdx].Name)
			}
			
			// Remove the connection
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			log.Info("Deleted connection '%s'", name)
			return c.Save()
		}
	}
	return stacktrace.NewError("Connection not found: %s", name)
}

// SetDefaultConnection sets a connection as the default
func (c *Config) SetDefaultConnection(name string) error {
	found := false
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			c.Connections[i].IsDefault = true
			found = true
			log.Info("Set '%s' as the default connection", name)
		} else {
			c.Connections[i].IsDefault = false
		}
	}
	
	if !found {
		return stacktrace.NewError("Connection not found: %s", name)
	}
	
	return c.Save()
}

// GetConnection retrieves a connection by name
func (c *Config) GetConnection(name string) *Connection {
	for _, conn := range c.Connections {
		if conn.Name == name {
			return &conn
		}
	}
	return nil
}