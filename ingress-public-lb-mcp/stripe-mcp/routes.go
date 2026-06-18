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
// authenticated MCP requests. Token presence is enforced by the shim at the
// load balancer — requests that reach this server always have a valid bearer token.
func newRouter(mcpServer *server.StreamableHTTPServer, aicIssuer string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// OAuth AS metadata (RFC 8414) — points agents directly to AIC's
		// endpoints for authorization, registration, and token exchange.
		if strings.HasPrefix(path, "/.well-known/oauth-authorization-server") ||
			strings.HasPrefix(path, "/.well-known/openid-configuration") {
			handleOAuthDiscovery(w, aicIssuer)
			return
		}

		// Protected resource metadata (RFC 9728) — points agents directly to AIC
		// as the authorization server.
		if strings.HasPrefix(path, "/.well-known/oauth-protected-resource") {
			handleProtectedResourceMetadata(w, aicIssuer)
			return
		}

		// Resolve the user's email from AIC userinfo.
		authHeader := r.Header.Get("Authorization")
		log.Printf("incoming request: method=%s path=%s", r.Method, path)

		email, err := fetchEmailFromUserinfo(authHeader, aicIssuer+"/userinfo")
		if err != nil {
			log.Printf("userinfo lookup failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"error":"invalid_token","error_description":"Could not resolve user email: %s"}`, err.Error())
			return
		}

		log.Printf("token valid: email=%q — forwarding to MCP handler", email)
		r = r.WithContext(context.WithValue(r.Context(), contextKeyEmail, email))
		mcpServer.ServeHTTP(w, r)
	})
}
