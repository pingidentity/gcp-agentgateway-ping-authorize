package main

import (
	"log"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetFlags(0)

	azureTenantID = requireEnv("AZURE_TENANT_ID")
	azureClientID = requireEnv("AZURE_CLIENT_ID")
	azureClientSecret = requireEnv("AZURE_CLIENT_SECRET")
	mcpRequiredScopes = scopesToJSON(requireEnv("MCP_REQUIRED_SCOPES"))
	mcpPort := requireEnv("MCP_SERVER_PORT")

	// Initialize MCP server with Entra provisioning tools and wrap in a
	// Streamable HTTP transport.
	mcpSrv := server.NewMCPServer("entra-mcp", "1.0.0",
		server.WithToolCapabilities(false),
	)
	registerEntraMcpTools(mcpSrv)
	mcpRouter := newRouter(server.NewStreamableHTTPServer(mcpSrv))
	if err := http.ListenAndServe(":"+mcpPort, mcpRouter); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
