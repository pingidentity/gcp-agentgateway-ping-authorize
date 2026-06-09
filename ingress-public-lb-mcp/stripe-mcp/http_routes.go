package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

// newRouter builds the HTTP handler that routes OAuth discovery and
// authenticated MCP requests. Discovery endpoints (/.well-known/) are served
// unauthenticated so clients can bootstrap the OAuth flow. All other requests
// require a valid bearer token, enforced by the shim at the load balancer.
func newRouter(mcpServer *server.StreamableHTTPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Protected resource metadata (RFC 9728) — the only discovery endpoint
		// required by the MCP spec. Tells clients where the authorization server
		// is and what scopes are needed.
		if strings.HasPrefix(path, "/.well-known/oauth-protected-resource") {
			handleProtectedResourceMetadata(w)
			return
		}

		// OAuth AS metadata (RFC 8414) — not required by the MCP spec, but served
		// here for backwards compatibility with clients that don't yet discover the
		// authorization server from the protected resource metadata.
		if strings.HasPrefix(path, "/.well-known/oauth-authorization-server") {
			handleOAuthDiscovery(w)
			return
		}

		// --- MCP endpoint (authenticated) ---
		// Only /mcp is forwarded by the shim; reject anything else.
		if path != "/mcp" {
			http.NotFound(w, r)
			return
		}

		// Resolve the caller's identity from AIC userinfo.
		authHeader := r.Header.Get("Authorization")
		email, err := resolveCallerEmail(authHeader)
		if err != nil {
			log.Printf("userinfo lookup failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"error":"invalid_token","error_description":"Could not resolve user email: %s"}`, err.Error())
			return
		}

		// Inject caller email into context so MCP tool handlers can identify the user.
		ctx := context.WithValue(r.Context(), ctxKeyCallerEmail, email)
		r = r.WithContext(ctx)
		mcpServer.ServeHTTP(w, r)
	})
}
