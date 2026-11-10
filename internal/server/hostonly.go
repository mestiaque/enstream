package server

import (
	"net/http"

	"enstream/internal/security"
)

// HostOnly limits a route to the computer ENStream runs on (see security.IsHost).
// Other devices get 403 HOST_ONLY.
func HostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !security.IsHost(r) {
			Fail(w, http.StatusForbidden, "HOST_ONLY", "This can only be changed on the computer that runs ENStream.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
