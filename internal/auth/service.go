package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	repo "enstream/internal/database/repositories"
	"enstream/internal/security"
	"enstream/internal/server"
)

const (
	CookieName    = "enstream_session"
	SessionTTL    = 30 * 24 * time.Hour
	pinSettingKey = "auth.pin_hash"
	touchEvery    = time.Minute
)

var (
	ErrWrongPIN   = errors.New("wrong PIN")
	ErrNotEnabled = errors.New("no PIN is set")
)

// LockedError: too many wrong PINs from this address.
type LockedError struct{ Wait time.Duration }

func (e LockedError) Error() string {
	return fmt.Sprintf("too many wrong attempts; try again in %s", e.Wait.Round(time.Second))
}

type Service struct {
	Store    *repo.Store
	Throttle *Throttle
	Log      *slog.Logger
	enabled  atomic.Bool
}

// Load reads whether a PIN is set (call once at start-up).
func (s *Service) Load(ctx context.Context) error {
	_, ok, err := s.Store.Setting(ctx, pinSettingKey)
	s.enabled.Store(ok)
	return err
}

func (s *Service) Enabled() bool { return s.enabled.Load() }

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) checkPIN(ctx context.Context, ip, pin string) error {
	if d := s.Throttle.Wait(ip); d > 0 {
		return LockedError{d}
	}
	hash, ok, err := s.Store.Setting(ctx, pinSettingKey)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotEnabled
	}
	if !VerifyPIN(pin, hash) {
		if d := s.Throttle.Fail(ip); d > 0 {
			s.Log.Warn("PIN locked out", "ip", ip, "seconds", int(d.Seconds()))
			return LockedError{d}
		}
		return ErrWrongPIN
	}
	s.Throttle.Success(ip)
	return nil
}

// startSession creates a session and sets its cookie.
func (s *Service) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	tok, err := newToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(SessionTTL)
	if err := s.Store.CreateSession(ctx, hashToken(tok), truncate(r.UserAgent(), 300), clientIP(r), exp.Unix()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: tok, Path: "/", Expires: exp, MaxAge: int(SessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	return nil
}

func clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Login checks the PIN and starts a session.
func (s *Service) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, pin string) error {
	if err := s.checkPIN(ctx, clientIP(r), pin); err != nil {
		return err
	}
	s.Log.Info("signed in with PIN", "ip", clientIP(r))
	return s.startSession(ctx, w, r)
}

// Session returns the caller's live session, renewing it (sliding 30 days).
func (s *Service) Session(r *http.Request) (repo.Session, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" || len(c.Value) > 100 {
		return repo.Session{}, false
	}
	sess, err := s.Store.SessionByHash(r.Context(), hashToken(c.Value))
	if err != nil {
		return repo.Session{}, false
	}
	if time.Since(time.Unix(sess.LastSeenAt, 0)) > touchEvery {
		_ = s.Store.TouchSession(r.Context(), sess.ID, clientIP(r), time.Now().Add(SessionTTL).Unix())
	}
	return sess, true
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) error {
	clearCookie(w)
	if c, err := r.Cookie(CookieName); err == nil {
		return s.Store.DeleteSessionByHash(r.Context(), hashToken(c.Value))
	}
	return nil
}

// SetPIN sets a first PIN (current ignored) or changes it (current required). Other
// sessions are signed out; the caller gets a fresh session so they stay signed in.
func (s *Service) SetPIN(ctx context.Context, w http.ResponseWriter, r *http.Request, current, next string) error {
	if s.Enabled() {
		if err := s.checkPIN(ctx, clientIP(r), current); err != nil {
			return err
		}
	}
	if err := ValidatePIN(next); err != nil {
		return err
	}
	hash, err := HashPIN(next)
	if err != nil {
		return err
	}
	if err := s.Store.SetSetting(ctx, pinSettingKey, hash); err != nil {
		return err
	}
	s.enabled.Store(true)
	if err := s.Store.DeleteSessionsExcept(ctx, 0); err != nil {
		return err
	}
	s.Log.Info("PIN set", "ip", clientIP(r))
	return s.startSession(ctx, w, r)
}

// RemovePIN turns the PIN off (needs the current PIN) and ends every session.
func (s *Service) RemovePIN(ctx context.Context, w http.ResponseWriter, r *http.Request, current string) error {
	if err := s.checkPIN(ctx, clientIP(r), current); err != nil {
		return err
	}
	return s.Reset(ctx, w)
}

// Reset removes the PIN without checking it (used by RemovePIN and the -reset-pin flag).
func (s *Service) Reset(ctx context.Context, w http.ResponseWriter) error {
	if err := s.Store.DeleteSetting(ctx, pinSettingKey); err != nil {
		return err
	}
	s.enabled.Store(false)
	if w != nil {
		clearCookie(w)
	}
	return s.Store.DeleteSessionsExcept(ctx, 0)
}

// open lists API paths that work without a session.
var open = map[string]bool{"/api/v1/health": true, "/api/v1/auth/status": true, "/api/v1/auth/login": true}

// Middleware requires a session for the API when a PIN is set. The page itself and its
// static files stay reachable so the sign-in screen can load.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Enabled() || open[r.URL.Path] || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.Session(r); !ok {
			server.Fail(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Enter the PIN to continue")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if ip, ok := security.ClientIP(r); ok {
		return ip.String()
	}
	return ""
}
