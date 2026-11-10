package config

import (
	"slices"
	"sync"
)

// Live holds the configuration that can change while ENStream runs (from the settings page).
// Readers get a copy; Update validates, saves and swaps atomically.
type Live struct {
	mu   sync.RWMutex
	c    Config
	file Server // server section as written in config.json (command-line overrides are not saved)
}

// NewLive wraps c. fileServer is the server section from the file, before -host/-port overrides.
func NewLive(c *Config, fileServer Server) *Live {
	return &Live{c: c.clone(), file: fileServer}
}

func (c Config) clone() Config {
	c.Media.Extensions = slices.Clone(c.Media.Extensions)
	c.Scanner.ExcludedDirectories = slices.Clone(c.Scanner.ExcludedDirectories)
	c.Security.AllowedNetworks = slices.Clone(c.Security.AllowedNetworks)
	return c
}

// Get returns a copy of the current configuration.
func (l *Live) Get() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.c.clone()
}

// Update applies fn to a copy; if the result is valid it is saved to disk and becomes current.
func (l *Live) Update(fn func(*Config) error) (Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	next := l.c.clone()
	if err := fn(&next); err != nil {
		return l.c.clone(), err
	}
	if err := next.Validate(); err != nil {
		return l.c.clone(), err
	}
	onDisk := next.clone()
	onDisk.Server = l.file
	if err := onDisk.Save(); err != nil {
		return l.c.clone(), err
	}
	l.c = next
	return next.clone(), nil
}
