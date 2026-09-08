package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

func newRouter(mcpServer *server.StreamableHTTPServer, validator *tokenValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if err := validator.verify(r.Context(), strings.TrimPrefix(auth, "Bearer ")); err != nil {
			log.Printf("[OrderStatusMCP] auth rejected: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		mcpServer.ServeHTTP(w, r)
	})
}
