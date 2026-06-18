package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// handleOAuthDiscovery serves OAuth 2.0 Authorization Server Metadata (RFC 8414).
// Ideally agents would fetch this directly from AIC (the authorization_servers URL
// in our protected resource metadata), but some clients (e.g. Claude) fetch it
// from the MCP server's origin instead. This endpoint works around that by
// pointing all endpoints directly to AIC.
func handleOAuthDiscovery(w http.ResponseWriter, aicIssuer string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"issuer": "%s",
		"authorization_endpoint": "%s/authorize",
		"token_endpoint": "%s/access_token",
		"introspection_endpoint": "%s/introspect",
		"registration_endpoint": "%s/register",
		"scopes_supported": ["openid", "profile", "email"],
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["client_secret_basic"]
	}`, aicIssuer, aicIssuer, aicIssuer, aicIssuer, aicIssuer)
}

// handleProtectedResourceMetadata serves RFC 9728 metadata pointing clients
// directly to AIC as the authorization server. The agent discovers AIC's
// registration, token, and authorize endpoints from AIC's own discovery doc.
func handleProtectedResourceMetadata(w http.ResponseWriter, aicIssuer string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"authorization_servers":["%s"],"scopes_supported":["openid","profile","email"]}`,
		aicIssuer,
	)
}

// fetchEmailFromUserinfo calls AIC's userinfo endpoint with the bearer token
// and returns the user's email. AIC includes email in userinfo (not the access
// token) when the email scope is granted.
func fetchEmailFromUserinfo(authHeader, userinfoURL string) (string, error) {
	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("userinfo call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return "", fmt.Errorf("decode userinfo: %w", err)
	}
	log.Printf("userinfo claims: %v", claims)

	for _, field := range []string{"email", "preferred_username", "sub"} {
		if v, ok := claims[field].(string); ok && strings.Contains(v, "@") {
			return v, nil
		}
	}
	return "", fmt.Errorf("no email found in userinfo; claims=%v", claims)
}
