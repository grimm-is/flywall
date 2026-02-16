// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package i18n

import (
	"net/http"
)

// Middleware extracts the Accept-Language header and injects a printer into the context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept-Language")
		tag := MatchLanguage(accept)
		p := NewPrinter(tag)

		ctx := WithPrinter(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
