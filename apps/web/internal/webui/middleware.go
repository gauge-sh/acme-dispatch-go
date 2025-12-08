package webui

import (
	"net/http"
)

// sameOriginPosts is the cross-site guard for a cookie-session app: modern
// browsers stamp Sec-Fetch-Site on every request, and a form post from
// another origin has no business here. Requests without the header (curl,
// old browsers, tests) pass through — the sessions cookie is SameSite=Lax
// anyway, so this is a second layer, not the only one.
func sameOriginPosts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			switch r.Header.Get("Sec-Fetch-Site") {
			case "", "same-origin", "none":
			default:
				http.Error(w, "cross-site requests are not accepted", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
