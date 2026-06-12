package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// mcpRequiredScopes are the scopes required to call this MCP server,
// expressed as a JSON array string. Set at startup from MCP_REQUIRED_SCOPES.
var mcpRequiredScopes string

// requireEnv returns the value of the named environment variable or fatally
// exits if it is not set.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return val
}

// scopesToJSON converts a space-separated scope string into a JSON array string.
// e.g. "openid pingone:provisioning" → `["openid","pingone:provisioning"]`
func scopesToJSON(scopes string) string {
	parts := strings.Fields(scopes)
	quoted := make([]string, len(parts))
	for i, s := range parts {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// entraIssuerURL returns the Microsoft Entra issuer URL for OAuth discovery.
func entraIssuerURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", azureTenantID)
}

// handleProtectedResourceMetadata serves OAuth Protected Resource Metadata
// (RFC 9728). Tells MCP clients which authorization server to use.
func handleProtectedResourceMetadata(w http.ResponseWriter) {
	issuer := entraIssuerURL()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"authorization_servers":["%s"],"scopes_supported":%s}`,
		issuer, mcpRequiredScopes,
	)
}

// handleOAuthDiscovery serves OAuth 2.0 Authorization Server Metadata
// (RFC 8414) for backwards compatibility.
func handleOAuthDiscovery(w http.ResponseWriter) {
	issuer := entraIssuerURL()
	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", azureTenantID)
	authEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", azureTenantID)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"issuer": "%s",
		"authorization_endpoint": "%s",
		"token_endpoint": "%s",
		"scopes_supported": ["openid","profile","email",%s],
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code","client_credentials","refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["client_secret_basic","client_secret_post"]
	}`, issuer, authEndpoint, tokenEndpoint, mcpRequiredScopes[1:len(mcpRequiredScopes)-1])
}
