package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"enstream/internal/auth"
	"enstream/internal/config"
	"enstream/internal/server"
)

type Settings struct {
	Live *config.Live
	Auth *auth.Service
	Log  *slog.Logger
	// Server is the address actually in use (the desktop app may pick another free port).
	Server func() config.Server
}

type settingsView struct {
	Scanner struct {
		AutoScan            bool     `json:"auto_scan"`
		ScanIntervalMinutes int      `json:"scan_interval_minutes"`
		WorkerCount         int      `json:"worker_count"`
		ExcludedDirectories []string `json:"excluded_directories"`
		IncludeHidden       bool     `json:"include_hidden"`
	} `json:"scanner"`
	Media struct {
		Extensions        []string `json:"extensions"`
		DefaultExtensions []string `json:"default_extensions"`
	} `json:"media"`
	Thumbnail config.Thumbnail `json:"thumbnail"`
	Backup    config.Backup    `json:"backup"`
	// Read-only here: changed in config.json, applied after a restart.
	Server   config.Server `json:"server"`
	Security struct {
		PINEnabled      bool     `json:"pin_enabled"`
		AllowedNetworks []string `json:"allowed_networks"`
	} `json:"security"`
}

func (h *Settings) view() settingsView {
	c := h.Live.Get()
	var v settingsView
	v.Scanner.AutoScan, v.Scanner.ScanIntervalMinutes, v.Scanner.WorkerCount = c.Scanner.AutoScan, c.Scanner.ScanIntervalMinutes, c.Scanner.WorkerCount
	v.Scanner.ExcludedDirectories, v.Scanner.IncludeHidden = c.Scanner.ExcludedDirectories, c.Scanner.IncludeHidden
	v.Media.Extensions, v.Media.DefaultExtensions = c.Media.Extensions, config.DefaultExtensions
	v.Thumbnail, v.Backup, v.Server = c.Thumbnail, c.Backup, c.Server
	if h.Server != nil {
		v.Server = h.Server()
	}
	v.Security.PINEnabled = h.Auth.Enabled()
	v.Security.AllowedNetworks = c.Security.AllowedNetworks
	if len(v.Security.AllowedNetworks) == 0 {
		v.Security.AllowedNetworks = config.DefaultAllowedNetworks
	}
	return v
}

func (h *Settings) Get(w http.ResponseWriter, r *http.Request) { server.OK(w, h.view()) }

// Update: PUT /settings with only the fields to change.
func (h *Settings) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scanner *struct {
			AutoScan            *bool     `json:"auto_scan"`
			ScanIntervalMinutes *int      `json:"scan_interval_minutes"`
			WorkerCount         *int      `json:"worker_count"`
			ExcludedDirectories *[]string `json:"excluded_directories"`
			IncludeHidden       *bool     `json:"include_hidden"`
		} `json:"scanner"`
		Media *struct {
			Extensions *[]string `json:"extensions"`
		} `json:"media"`
		Thumbnail *struct {
			Width   *int `json:"width"`
			Quality *int `json:"quality"`
		} `json:"thumbnail"`
		Backup *struct {
			Auto *bool `json:"auto"`
			Keep *int  `json:"keep"`
		} `json:"backup"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send the settings to change as JSON (server and network settings are changed in config.json)")
		return
	}
	rescan := false
	_, err := h.Live.Update(func(c *config.Config) error {
		if s := body.Scanner; s != nil {
			set(&c.Scanner.AutoScan, s.AutoScan)
			set(&c.Scanner.ScanIntervalMinutes, s.ScanIntervalMinutes)
			set(&c.Scanner.WorkerCount, s.WorkerCount)
			if s.IncludeHidden != nil && *s.IncludeHidden != c.Scanner.IncludeHidden {
				rescan = true
			}
			set(&c.Scanner.IncludeHidden, s.IncludeHidden)
			if s.ExcludedDirectories != nil {
				dirs, err := cleanList(*s.ExcludedDirectories, 100, func(d string) (string, error) {
					if strings.ContainsAny(d, `/\`) {
						return "", errors.New("excluded folders are names like \"Samples\", not paths")
					}
					return d, nil
				})
				if err != nil {
					return err
				}
				rescan = rescan || !slices.Equal(dirs, c.Scanner.ExcludedDirectories)
				c.Scanner.ExcludedDirectories = dirs
			}
		}
		if m := body.Media; m != nil && m.Extensions != nil {
			exts, err := cleanList(*m.Extensions, 64, func(e string) (string, error) {
				e = strings.ToLower(e)
				if !strings.HasPrefix(e, ".") {
					e = "." + e
				}
				if len(e) < 2 || len(e) > 10 || strings.ContainsAny(e[1:], `./\ `) {
					return "", errors.New("extensions look like mp4 or .mkv")
				}
				return e, nil
			})
			if err != nil {
				return err
			}
			rescan = rescan || !slices.Equal(exts, c.Media.Extensions)
			c.Media.Extensions = exts
		}
		if t := body.Thumbnail; t != nil {
			set(&c.Thumbnail.Width, t.Width)
			set(&c.Thumbnail.Quality, t.Quality)
		}
		if b := body.Backup; b != nil {
			set(&c.Backup.Auto, b.Auto)
			set(&c.Backup.Keep, b.Keep)
		}
		return nil
	})
	if err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_SETTINGS", friendly(err))
		return
	}
	msg := "Settings saved"
	if rescan {
		msg = "Settings saved; scan your folders to apply them to the library"
	}
	h.Log.Info("settings changed")
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Data: h.view(), Message: msg})
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// cleanList trims, normalises, de-duplicates and limits a list from the UI.
func cleanList(in []string, max int, norm func(string) (string, error)) ([]string, error) {
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		v, err := norm(s)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if len(out) > max {
		return nil, errors.New("too many entries")
	}
	return out, nil
}

// friendly turns "scanner.worker_count must be..." into a sentence for people.
func friendly(err error) string {
	r := strings.NewReplacer("scanner.worker_count", "Parallel workers", "scanner.scan_interval_minutes", "Scan interval (minutes)",
		"thumbnail.width", "Thumbnail width", "thumbnail.quality", "Thumbnail quality", "backup.keep", "Backups to keep",
		"media.extensions", "Video formats", "scanner.excluded_directories", "Excluded folders")
	msg := r.Replace(err.Error())
	if msg != "" {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return msg
}
