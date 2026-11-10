package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsAreValid(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(c.AllowedPrefixes()) != len(DefaultAllowedNetworks) {
		t.Fatal("empty allowed_networks must mean the private defaults")
	}
}

func TestValidationRejectsBadValues(t *testing.T) {
	cases := map[string]func(*Config){
		"port":      func(c *Config) { c.Server.Port = 70000 },
		"host":      func(c *Config) { c.Server.Host = "my pc" },
		"workers":   func(c *Config) { c.Scanner.WorkerCount = 0 },
		"width":     func(c *Config) { c.Thumbnail.Width = 10 },
		"quality":   func(c *Config) { c.Thumbnail.Quality = 101 },
		"extension": func(c *Config) { c.Media.Extensions = []string{"mp4"} },
		"network":   func(c *Config) { c.Security.AllowedNetworks = []string{"192.168.1.1/40"} },
		"excluded":  func(c *Config) { c.Scanner.ExcludedDirectories = []string{" "} },
	}
	for name, mutate := range cases {
		c := Default()
		mutate(&c)
		if c.Validate() == nil {
			t.Errorf("%s: invalid value accepted", name)
		}
	}
}

func TestLoadCreatesFillsAndKeepsUserValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 8080 {
		t.Fatalf("default port: %d", c.Server.Port)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("a missing config file is created")
	}
	// A partial file: given keys kept, missing keys take defaults.
	if err := os.WriteFile(path, []byte(`{"server":{"port":9000},"scanner":{"worker_count":5}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 9000 || c.Scanner.WorkerCount != 5 || c.Thumbnail.Width != 480 {
		t.Fatalf("merge: %+v", c)
	}
	// An invalid file is an error, not silently replaced.
	if err := os.WriteFile(path, []byte(`{"server":{"port":0}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "server.port") {
		t.Fatalf("invalid file: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"port":0`) {
		t.Fatal("the invalid file must be left as the user wrote it")
	}
}

func TestPathsAndExtensions(t *testing.T) {
	p, err := NewPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{p.Thumbnails, p.Subtitles, p.Logs, p.Backups} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Fatalf("%s not created", d)
		}
	}
	c := Default()
	c.Media.Extensions = []string{".MKV"}
	if !c.ExtensionSet()[".mkv"] {
		t.Fatal("extensions are case-insensitive")
	}
}
