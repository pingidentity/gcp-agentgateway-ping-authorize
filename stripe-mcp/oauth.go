package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// pingOneAicIssuer is the AIC OAuth 2.0 issuer URL, set once at startup from PING_AIC_ISSUER.
var pingOneAicIssuer string

// oauthScopesJSON is the JSON array representation of OAUTH_SCOPES, e.g. ["openid","profile","email", "stripe_mcp:invoke"].
var oauthScopesJSON string

// handleProtectedResourceMetadata serves OAuth Protected Resource Metadata (RFC 9728).
// This is the only discovery endpoint required by the MCP spec. It tells clients
// which authorization server to use (AIC) and what scopes are required.
func handleProtectedResourceMetadata(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"authorization_servers":["%s"],"scopes_supported":%s}`,
		pingOneAicIssuer, oauthScopesJSON,
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
		"scopes_supported": %s,
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["client_secret_basic"]
	}`, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, pingOneAicIssuer, oauthScopesJSON)
}

// resolveCallerEmail exchanges the caller's bearer token for their email address
// by calling AIC's userinfo endpoint. The email is used to look up the caller's
// Stripe customer record.
func resolveCallerEmail(bearerToken string) (string, error) {
	url := pingOneAicIssuer + "/userinfo"
	log.Printf("resolveCallerEmail: calling %s with token=%s", url, bearerToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", bearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("userinfo call failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("resolveCallerEmail: status=%d body=%s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return "", fmt.Errorf("failed to decode userinfo response: %w", err)
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("no email claim in userinfo response: %v", claims)
	}

	log.Printf("resolveCallerEmail: resolved email=%s", email)
	return email, nil
}
