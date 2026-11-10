// Package config loads, validates and saves ENStream's configuration file (config.json in the
// data directory). Unknown or missing keys fall back to defaults; the file is never rewritten
// with values the user did not choose, except to add new keys with their defaults.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Scanner struct {
	WorkerCount         int      `json:"worker_count"`
	AutoScan            bool     `json:"auto_scan"`
	ScanIntervalMinutes int      `json:"scan_interval_minutes"`
	ExcludedDirectories []string `json:"excluded_directories"`
	IncludeHidden       bool     `json:"include_hidden"`
}

type Thumbnail struct {
	Width   int `json:"width"`
	Quality int `json:"quality"`
}

type Media struct {
	Extensions []string `json:"extensions"`
	FFmpegPath string   `json:"ffmpeg_path"`
	FFprobePath string  `json:"ffprobe_path"`
}

type Security struct {
	AuthenticationEnabled bool `json:"authentication_enabled"`
	// CIDR ranges allowed to connect. Empty = loopback plus the private ranges
	// (RFC 1918, link-local, IPv6 ULA); see DefaultAllowedNetworks.
	AllowedNetworks []string `json:"allowed_networks"`
}

type Config struct {
	Server    Server    `json:"server"`
	Media     Media     `json:"media"`
	Scanner   Scanner   `json:"scanner"`
	Thumbnail Thumbnail `json:"thumbnail"`
	Security  Security  `json:"security"`

	// Not stored: where the file lives.
	path string
}

// DefaultExtensions are the video extensions the scanner considers (case-insensitive).
var DefaultExtensions = []string{
	".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".mpeg", ".mpg", ".wmv", ".flv",
	".3gp", ".ts", ".mts", ".m2ts", ".ogv", ".vob",
}

// DefaultAllowedNetworks: this computer and private home/office networks. Public addresses
// are refused unless the user lists them explicitly.
var DefaultAllowedNetworks = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "fe80::/10", "fc00::/7",
	"100.64.0.0/10", // carrier-grade NAT / mesh VPNs such as Tailscale
}

func Default() Config {
	return Config{
		Server:    Server{Host: "0.0.0.0", Port: 8080},
		Media:     Media{Extensions: append([]string(nil), DefaultExtensions...)},
		Scanner:   Scanner{WorkerCount: 3, ScanIntervalMinutes: 60, ExcludedDirectories: []string{}},
		Thumbnail: Thumbnail{Width: 480, Quality: 80},
		Security:  Security{AllowedNetworks: []string{}},
	}
}

// Validate checks every value a user can edit; the first problem is returned.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535 (got %d)", c.Server.Port)
	}
	if c.Server.Host != "" {
		if _, err := netip.ParseAddr(c.Server.Host); err != nil && c.Server.Host != "localhost" {
			return fmt.Errorf("server.host must be an IP address such as 0.0.0.0 (got %q)", c.Server.Host)
		}
	}
	if c.Scanner.WorkerCount < 1 || c.Scanner.WorkerCount > 16 {
		return fmt.Errorf("scanner.worker_count must be between 1 and 16 (got %d)", c.Scanner.WorkerCount)
	}
	if c.Scanner.ScanIntervalMinutes < 5 || c.Scanner.ScanIntervalMinutes > 7*24*60 {
		return fmt.Errorf("scanner.scan_interval_minutes must be between 5 and 10080")
	}
	if c.Thumbnail.Width < 120 || c.Thumbnail.Width > 1920 {
		return fmt.Errorf("thumbnail.width must be between 120 and 1920 (got %d)", c.Thumbnail.Width)
	}
	if c.Thumbnail.Quality < 1 || c.Thumbnail.Quality > 100 {
		return fmt.Errorf("thumbnail.quality must be between 1 and 100 (got %d)", c.Thumbnail.Quality)
	}
	if len(c.Media.Extensions) == 0 {
		return errors.New("media.extensions cannot be empty")
	}
	for _, e := range c.Media.Extensions {
		if !strings.HasPrefix(e, ".") || len(e) < 2 || strings.ContainsAny(e, `/\ `) {
			return fmt.Errorf("media.extensions: %q is not an extension like .mp4", e)
		}
	}
	for _, n := range c.Security.AllowedNetworks {
		if _, err := netip.ParsePrefix(n); err != nil {
			return fmt.Errorf("security.allowed_networks: %q is not a network like 192.168.1.0/24", n)
		}
	}
	for _, d := range c.Scanner.ExcludedDirectories {
		if strings.TrimSpace(d) == "" {
			return errors.New("scanner.excluded_directories cannot contain empty names")
		}
	}
	return nil
}

// AllowedPrefixes parses the allowed networks (defaults when none are configured).
func (c *Config) AllowedPrefixes() []netip.Prefix {
	src := c.Security.AllowedNetworks
	if len(src) == 0 {
		src = DefaultAllowedNetworks
	}
	out := make([]netip.Prefix, 0, len(src))
	for _, s := range src {
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p.Masked())
		}
	}
	return out
}

// ExtensionSet returns the lower-cased extensions for fast lookups.
func (c *Config) ExtensionSet() map[string]bool {
	m := make(map[string]bool, len(c.Media.Extensions))
	for _, e := range c.Media.Extensions {
		m[strings.ToLower(e)] = true
	}
	return m
}

func (c *Config) Path() string { return c.path }

// Load reads path, filling missing keys with defaults. A missing file is created with the
// defaults; an unreadable or invalid file is an error (it is never silently replaced).
func Load(path string) (*Config, error) {
	c := Default()
	c.path = path
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := c.Save(); err != nil {
			return nil, err
		}
		return &c, nil
	case err != nil:
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config %s is not valid JSON: %w", path, err)
	}
	c.path = path
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the configuration atomically (temporary file, then rename).
func (c *Config) Save() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
