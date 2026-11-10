package handlers

import (
	"enstream/internal/security"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"enstream/internal/auth"
	"enstream/internal/server"
)

type Auth struct {
	Service *auth.Service
	Log     *slog.Logger
}

func (h *Auth) fail(w http.ResponseWriter, r *http.Request, err error) {
	var locked auth.LockedError
	switch {
	case errors.As(err, &locked):
		w.Header().Set("Retry-After", strconv.Itoa(int(locked.Wait.Seconds())+1))
		server.Fail(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", locked.Error())
	case errors.Is(err, auth.ErrWrongPIN):
		server.Fail(w, http.StatusUnauthorized, "WRONG_PIN", "That PIN is not correct")
	case errors.Is(err, auth.ErrNotEnabled):
		server.Fail(w, http.StatusConflict, "PIN_NOT_SET", "No PIN is set")
	case errors.Is(err, auth.ErrWeakPIN):
		server.Fail(w, http.StatusBadRequest, "INVALID_PIN", "The PIN must have "+auth.ErrWeakPIN.Error())
	default:
		internal(w, r, h.Log, err)
	}
}

func (h *Auth) Status(w http.ResponseWriter, r *http.Request) {
	_, ok := h.Service.Session(r)
	enabled := h.Service.Enabled()
	// "host": the computer ENStream runs on (library management); other devices only watch.
	server.OK(w, map[string]bool{"pin_enabled": enabled, "authenticated": !enabled || ok, "host": security.IsHost(r)})
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PIN string `json:"pin"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {\"pin\": \"...\"}")
		return
	}
	if err := h.Service.Login(r.Context(), w, r, body.PIN); err != nil {
		h.fail(w, r, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Signed in"})
}

func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Service.Logout(w, r); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Signed out"})
}

// SetPIN: PUT /auth/pin {"current_pin": "...", "new_pin": "..."} (current only when changing).
func (h *Auth) SetPIN(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current_pin"`
		New     string `json:"new_pin"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {\"new_pin\": \"...\"}")
		return
	}
	was := h.Service.Enabled()
	if err := h.Service.SetPIN(r.Context(), w, r, body.Current, body.New); err != nil {
		h.fail(w, r, err)
		return
	}
	msg := "PIN turned on. Other devices must enter it."
	if was {
		msg = "PIN changed. Other devices were signed out."
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: msg})
}

// DisablePIN: POST /auth/pin/disable {"current_pin": "..."}.
func (h *Auth) DisablePIN(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current_pin"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {\"current_pin\": \"...\"}")
		return
	}
	if err := h.Service.RemovePIN(r.Context(), w, r, body.Current); err != nil {
		h.fail(w, r, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "PIN turned off"})
}

func (h *Auth) Sessions(w http.ResponseWriter, r *http.Request) {
	list, err := h.Service.Store.Sessions(r.Context())
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	if cur, ok := h.Service.Session(r); ok {
		for i := range list {
			list[i].Current = list[i].ID == cur.ID
		}
	}
	server.OK(w, list)
}

func (h *Auth) DeleteSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.Store.DeleteSession(r.Context(), id); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Signed out that device"})
}

func (h *Auth) SignOutOthers(w http.ResponseWriter, r *http.Request) {
	cur, _ := h.Service.Session(r)
	if err := h.Service.Store.DeleteSessionsExcept(r.Context(), cur.ID); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Other devices were signed out"})
}
