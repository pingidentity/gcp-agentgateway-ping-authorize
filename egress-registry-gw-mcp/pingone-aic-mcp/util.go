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

// getEnvOrDefault returns the value of the named environment variable, or
// defaultValue if it is not set.
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
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

// aicIssuerURL returns the AIC OAuth 2.0 issuer URL derived from AIC_BASE_URL
// and AIC_REALM. Used only for OAuth discovery responses.
func aicIssuerURL() string {
	return fmt.Sprintf("%s/am/oauth2/%s", aicBaseURL, aicRealm)
}

// handleProtectedResourceMetadata serves OAuth Protected Resource Metadata
// (RFC 9728). Tells MCP clients which authorization server to use and what
// scopes are required to call this MCP server.
func handleProtectedResourceMetadata(w http.ResponseWriter) {
	issuer := aicIssuerURL()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"authorization_servers":["%s"],"scopes_supported":%s}`,
		issuer, mcpRequiredScopes,
	)
}

// handleOAuthDiscovery serves OAuth 2.0 Authorization Server Metadata
// (RFC 8414) for backwards compatibility.
func handleOAuthDiscovery(w http.ResponseWriter) {
	issuer := aicIssuerURL()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"issuer": "%s",
		"authorization_endpoint": "%s/authorize",
		"token_endpoint": "%s/access_token",
		"introspection_endpoint": "%s/introspect",
		"scopes_supported": ["openid","profile",%s],
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code","client_credentials","refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["client_secret_basic","client_secret_post"]
	}`, issuer, issuer, issuer, issuer, mcpRequiredScopes[1:len(mcpRequiredScopes)-1])
}
