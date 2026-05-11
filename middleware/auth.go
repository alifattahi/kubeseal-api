package middleware

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth wraps a handler with HTTP Basic Authentication.
func BasicAuth(expectedUser, expectedPass string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok || !secureCompare(user, expectedUser) || !secureCompare(pass, expectedPass) {
				w.Header().Set("WWW-Authenticate", `Basic realm="sealed-secret-api"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// secureCompare does constant-time string comparison to prevent timing attacks.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
