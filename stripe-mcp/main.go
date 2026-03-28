package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	stripe "github.com/stripe/stripe-go/v79"
)

func main() {
	stripe.Key = requireEnv("STRIPE_SECRET_KEY")
	pingOneAicIssuer = strings.TrimSuffix(requireEnv("PING_AIC_ISSUER"), "/")
	oauthScopesJSON = scopesToJSON(requireEnv("OAUTH_SCOPES"))
	port := "8080"

	// Initialize the MCP server with Stripe tools and wrap it in a Streamable HTTP transport.
	mcpSrv := server.NewMCPServer("stripe-mcp", "1.0.0",
		server.WithToolCapabilities(false),
	)
	registerStripeMcpTools(mcpSrv)
	handler := newRouter(server.NewStreamableHTTPServer(mcpSrv))

	log.Printf("stripe-mcp listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return val
}

// scopesToJSON converts a space-separated scope string (e.g. "openid profile email stripe_mcp:invoke")
// into a JSON array string (e.g. `["openid","profile","email","stripe_mcp:invoke"]`).
func scopesToJSON(scopes string) string {
	parts := strings.Fields(scopes)
	quoted := make([]string, len(parts))
	for i, s := range parts {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
