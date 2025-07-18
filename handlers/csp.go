package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/mtlynch/picoshare/v2/build" // Import the build package
	"github.com/mtlynch/picoshare/v2/random"
)

type contextKey struct {
	name string
}

var contextKeyCSPNonce = &contextKey{"csp-nonce"}

func enforceContentSecurityPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := base64.StdEncoding.EncodeToString(random.Bytes(16))

		// Default script-src values.
		scriptSrcValues := []string{
			"'self'",
		}

		// In development, we need 'unsafe-inline' for air's live-reloader.
		// In production, we use a much stricter nonce-based policy.
		// The browser IGNORES 'unsafe-inline' if a nonce is present, so we must use one or the other.
		if build.IsDevBuild {
			scriptSrcValues = append(scriptSrcValues, "'unsafe-inline'")
		} else {
			scriptSrcValues = append(scriptSrcValues, "'nonce-"+nonce+"'")
		}

		type cspDirective struct {
			name   string
			values []string
		}
		directives := []cspDirective{
			{
				name: "default-src",
				values: []string{
					"'self'",
				},
			},
			{
				name:   "script-src",
				values: scriptSrcValues,
			},
			{
				name: "style-src-elem",
				values: []string{
					"'self'",
					// Firefox refuses to load an inline <style> tag in an HTML custom
					// element, even if we specify a nonce:
					// https://github.com/mtlynch/picoshare/issues/249
					"'unsafe-inline'",
				},
			},
			{
				name: "media-src",
				values: []string{
					"'self'",
					// For some reason, Firefox throws an error if we don't allow data in
					// as a media-src, even on pages where there are no video, audio, or
					// track tags.
					"data:",
				},
			},
		}
		policyParts := []string{}
		for _, directive := range directives {
			policyParts = append(policyParts, fmt.Sprintf("%s %s", directive.name, strings.Join(directive.values, " ")))
		}
		policy := strings.Join(policyParts, "; ") + ";"

		w.Header().Set("Content-Security-Policy", policy)

		ctx := context.WithValue(r.Context(), contextKeyCSPNonce, nonce)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func cspNonce(ctx context.Context) string {
	key, ok := ctx.Value(contextKeyCSPNonce).(string)
	if !ok {
		panic("CSP nonce is missing from request context")
	}
	return key
}