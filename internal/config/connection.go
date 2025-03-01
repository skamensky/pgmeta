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
)

type Connection struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	IsDefault bool  `json:"is_default"`
}

type Config struct {
	Connections []Connection `json:"connections"`
	configPath  string
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".pgmeta")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	cfg := &Config{configPath: configPath}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(c.configPath, data, 0644)
}

func (c *Config) AddConnection(name, url string, makeDefault bool) error {
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

		// Convert to connection string
		connStr, err := pq.ParseURL(url)
		if err != nil {
			return stacktrace.Propagate(err, "Invalid connection URL")
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
		connParams = append(connParams, fmt.Sprintf("%s=%s", k, v))
	}
	url = strings.Join(connParams, " ")

	// If this is the first connection, make it default
	if len(c.Connections) == 0 {
		makeDefault = true
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

	return c.Save()
}

func (c *Config) GetDefaultConnection() *Connection {
	for _, conn := range c.Connections {
		if conn.IsDefault {
			return &conn
		}
	}
	if len(c.Connections) == 1 {
		return &c.Connections[0]
	}
	return nil
}

func (c *Config) DeleteConnection(name string) error {
	for i, conn := range c.Connections {
		if conn.Name == name {
			// If deleting default connection, clear default status
			if conn.IsDefault && len(c.Connections) > 1 {
				// Make the next connection default, or the previous if this is the last one
				nextIdx := (i + 1) % len(c.Connections)
				c.Connections[nextIdx].IsDefault = true
			}
			
			// Remove the connection
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			return c.Save()
		}
	}
	return stacktrace.NewError("Connection not found: %s", name)
}

func (c *Config) SetDefaultConnection(name string) error {
	found := false
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			c.Connections[i].IsDefault = true
			found = true
		} else {
			c.Connections[i].IsDefault = false
		}
	}
	if !found {
		return stacktrace.NewError("Connection not found: %s", name)
	}
	return c.Save()
}

func (c *Config) GetConnection(name string) *Connection {
	for _, conn := range c.Connections {
		if conn.Name == name {
			return &conn
		}
	}
	return nil
} 