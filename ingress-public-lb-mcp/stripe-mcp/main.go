package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	stripe "github.com/stripe/stripe-go/v79"
)

func main() {
	log.SetFlags(0)

	stripe.Key = requireEnv("STRIPE_SECRET_KEY")
	pingOneAicIssuer = strings.TrimSuffix(requireEnv("PINGONE_AIC_ISSUER"), "/")
	mcpRequiredScopes = scopesToJSON(requireEnv("MCP_REQUIRED_SCOPES"))
	mcpPort := requireEnv("MCP_SERVER_PORT")

	// Initialize the MCP server with Stripe tools and wrap it in a Streamable HTTP transport.
	mcpSrv := server.NewMCPServer("stripe-mcp", "1.0.0",
		server.WithToolCapabilities(false),
	)
	registerStripeMcpTools(mcpSrv)
	mcpRouter := newRouter(server.NewStreamableHTTPServer(mcpSrv))
	if err := http.ListenAndServe(":"+mcpPort, mcpRouter); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
