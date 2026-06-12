package main

import (
	"log"
	"net/http"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetFlags(0)

	aicBaseURL = requireEnv("AIC_BASE_URL")
	aicAdminClientID = requireEnv("AIC_ADMIN_CLIENT_ID")
	aicAdminClientSecret = requireEnv("AIC_ADMIN_CLIENT_SECRET")
	aicRealm = getEnvOrDefault("AIC_REALM", "alpha")
	mcpRequiredScopes = scopesToJSON(requireEnv("MCP_REQUIRED_SCOPES"))
	mcpPort := requireEnv("MCP_SERVER_PORT")

	// Initialize MCP server with AIC provisioning tools and wrap in a
	// Streamable HTTP transport.
	mcpSrv := server.NewMCPServer("pingone-aic-mcp", "1.0.0",
		server.WithToolCapabilities(false),
	)
	registerAicMcpTools(mcpSrv)
	mcpRouter := newRouter(server.NewStreamableHTTPServer(mcpSrv))
	if err := http.ListenAndServe(":"+mcpPort, mcpRouter); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
