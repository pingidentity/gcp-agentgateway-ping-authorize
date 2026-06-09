package main

import (
	"fmt"
	"net/http"
)

// pingOneAicIssuer is the AIC OAuth 2.0 issuer URL, set once at startup from PINGONE_AIC_ISSUER.
var pingOneAicIssuer string

// mcpRequiredScopes are the scopes needed to invoke this MCP server, set once at startup from MCP_REQUIRED_SCOPES.
var mcpRequiredScopes string

// handleProtectedResourceMetadata serves OAuth Protected Resource Metadata (RFC 9728).
// This is the only discovery endpoint required by the MCP spec. It tells clients
// which authorization server to use (AIC) and what scopes are required.
func handleProtectedResourceMetadata(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"authorization_servers":["%s"],"scopes_supported":%s}`,
		pingOneAicIssuer, mcpRequiredScopes,
	)
}

// handleOAuthDiscovery serves OAuth 2.0 Authorization Server Metadata (RFC 8414).
// Not required by the MCP spec — agents should discover AIC via the
// authorization_servers URL in the protected resource metadata. Served here for
// backwards compatibility with clients that expect this on the MCP server's origin.
func handleOAuthDiscovery(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"issuer": "%s",
		"authorization_endpoint": "%s/authorize",
		"token_endpoint": "%s/access_token",
		"introspection_endpoint": "%s/introspect",
		"registration_endpoint": "%s/register",
		"scopes_supported": ["openid","profile",%s],
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["client_secret_basic"]
	}`, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, mcpRequiredScopes[1:len(mcpRequiredScopes)-1])
}
