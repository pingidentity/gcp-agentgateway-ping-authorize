package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

// newRouter builds the HTTP handler that routes OAuth discovery and
// authenticated MCP requests.
//
// The Agent Gateway (internal LB + ext_proc shim) has already validated the
// bearer token before traffic reaches this server. The router handles
// discovery endpoints and forwards /mcp requests to the MCP protocol handler.
func newRouter(mcpServer *server.StreamableHTTPServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Protected resource metadata (RFC 9728) — tells clients which
		// authorization server issues tokens for this server.
		if strings.HasPrefix(path, "/.well-known/oauth-protected-resource") {
			handleProtectedResourceMetadata(w)
			return
		}

		// OAuth AS metadata (RFC 8414) — served for backwards compatibility.
		if strings.HasPrefix(path, "/.well-known/oauth-authorization-server") {
			handleOAuthDiscovery(w)
			return
		}

		// Accept /mcp or /mcp/pingone (after optional LB path rewrite).
		// The shim has already enforced authentication; log the caller for audit.
		if path == "/mcp" || path == "/mcp/pingone" || strings.HasPrefix(path, "/mcp/pingone/") {
			authHeader := r.Header.Get("Authorization")
			log.Printf("mcp request from bearer=%s path=%s method=%s",
				truncateToken(authHeader), path, r.Method)

			// Normalise path to /mcp so the mcp-go server can handle it.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/mcp"
			mcpServer.ServeHTTP(w, r2)
			return
		}

		log.Printf("rejected unknown path: %s", path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"not_found","error_description":"Unknown path"}`)
	})
}

// truncateToken returns the last 8 chars of a bearer token for safe logging.
func truncateToken(authHeader string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "<none>"
	}
	token := authHeader[len(prefix):]
	if len(token) <= 8 {
		return "****"
	}
	return "..." + token[len(token)-8:]
}
