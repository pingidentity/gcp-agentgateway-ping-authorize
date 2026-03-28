package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

// PingAuthzShim is the ext_proc gRPC server. The load balancer / agent gateway calls this for
// every intercepted request, and forwards the request headers as attributes to PingAuthorize
// to get an allow/deny decision to send back to the load balancer / agent gateway before the
// request reaches the downstream mcp server.
type PingAuthzShim struct {
	httpClient       *http.Client
	pingAuthorizeURL string
	mcpServerURL     string
	scopes           string
}

// NewPingAuthzShim initializes the ext_proc shim from environment configuration.
// It wires up the HTTP client used to call PingAuthorize and the MCP server,
// with optional TLS verification bypass for development environments.
func NewPingAuthzShim() *PingAuthzShim {
	return &PingAuthzShim{
		pingAuthorizeURL: strings.TrimSpace(os.Getenv("PING_AUTHORIZE_URL")),
		mcpServerURL:     strings.TrimSpace(os.Getenv("MCP_SERVER_URL")),
		scopes:           strings.TrimSpace(os.Getenv("OAUTH_SCOPES")),
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: newTLSTransport(os.Getenv("SKIP_TLS_VERIFY") == "true"),
		},
	}
}

// Process handles the bidirectional gRPC stream between Envoy and this ext_proc
// shim. Envoy emits a message per processing phase (request headers, body, etc.)
// and blocks until the shim responds for each phase it owns.
//
// Only the RequestHeaders phase is intercepted. A deny decision short-circuits
// the entire request — Envoy returns an immediate error response to the client
// and the downstream service never sees the request. Unhandled phases are ignored,
// allowing Envoy to proceed with its default behavior.
func (s *PingAuthzShim) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("ext_proc stream error: %v", err)
			return err
		}

		reqHeaders := msg.GetRequestHeaders()
		if reqHeaders == nil {
			continue
		}

		if err := stream.Send(s.evaluateRequest(reqHeaders)); err != nil {
			return err
		}
	}
}

// evaluateRequest evaluates a single inbound request and returns one of:
//   - 401 with WWW-Authenticate directing the agent to protected resource metadata (/.well-known/oauth-protected-resource) if no bearer token is present
//   - Passthrough for public OAuth endpoints (/.well-known/**) that agents need to access without a token
//   - CONTINUE (allow) or 403 (deny) based on the PingAuthorize policy decision
func (s *PingAuthzShim) evaluateRequest(headers *extproc.HttpHeaders) *extproc.ProcessingResponse {
	traceID := fmt.Sprintf("pa-%d", time.Now().UnixNano())
	requestHeaders := flattenEnvoyHeaders(headers)
	path := requestHeaders[":path"]

	// Discovery endpoints are always allowed — agents need them to find the authorization server.
	if strings.HasPrefix(path, "/.well-known") {
		log.Printf("[%s] passthrough path=%q", traceID, path)
		return buildPassthroughResponse()
	}

	// Only /.well-known/** and /mcp are valid paths.
	if path != "/mcp" {
		log.Printf("[%s] unknown path=%q, returning 404", traceID, path)
		return buildRejectResponse(
			typev3.StatusCode_NotFound, "", "",
			`{"error":"not_found","error_description":"Unknown path"}`,
		)
	}

	// No bearer token → 401 so the agent can discover and start the OAuth flow to get a token.
	if ExtractBearerToken(requestHeaders["authorization"]) == "" {
		log.Printf("[%s] no bearer token, returning 401", traceID)
		return buildRejectResponse(
			typev3.StatusCode_Unauthorized, s.getWWWAuthenticate(), "",
			`{"error":"unauthorized","error_description":"Bearer token required"}`,
		)
	}

	// Forward all headers as policy attributes to PingAuthorize and receive a decision.
	req := pingAuthorizeRequest{Attributes: requestHeaders}
	allowed, decision, err := s.callPingAuthorize(req)
	if err != nil {
		log.Printf("[%s] policy call failed: %v", traceID, err)
		return buildRejectResponse(
			typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
			`{"error":"access_denied","decision_id":"`+traceID+`","source":"ping-authorize"}`,
		)
	}
	log.Printf("[%s] path=%q decision=%s", traceID, path, decision.Decision)

	// Return an allow or deny response based on the policy decision. Include the
	// www-authenticate header to guide the agent where to get a valid token.
	if allowed {
		return buildAllowResponse(traceID)
	}
	return buildRejectResponse(
		typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
		`{"error":"access_denied","decision":"deny","decision_id":"`+traceID+`","source":"ping-authorize"}`,
	)
}

// getWWWAuthenticate returns the WWW-Authenticate header value for reject responses,
// pointing MCP clients to the protected resource metadata for OAuth discovery.
func (s *PingAuthzShim) getWWWAuthenticate() string {
	return fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource", scope="%s"`, s.mcpServerURL, s.scopes)
}
