package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

// PingAuthzShim is the ext_proc gRPC server. The load balancer / agent gateway calls this for
// every intercepted request. It inspects both headers and body to extract policy attributes
// (bearer token, MCP tool name, purchase amount) and forwards them to PingAuthorize for an
// allow/deny decision before the request reaches the downstream MCP server.
type PingAuthzShim struct {
	httpClient        *http.Client
	pingAuthorizeURL  string
	mcpServerURL      string
	mcpRequiredScopes string
}

// NewPingAuthzShim creates the ext_proc shim with the given configuration.
func NewPingAuthzShim(pingAuthorizeURL, mcpServerURL, mcpRequiredScopes string, skipTLSVerify bool) *PingAuthzShim {
	return &PingAuthzShim{
		pingAuthorizeURL:  pingAuthorizeURL,
		mcpServerURL:      mcpServerURL,
		mcpRequiredScopes: mcpRequiredScopes,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: newTLSTransport(skipTLSVerify),
		},
	}
}

// Process handles the bidirectional gRPC stream from the Agent Gateway.
//
// Two phases are intercepted:
//  1. RequestHeaders — fast-path decisions (passthrough, 401, 404). For authenticated
//     /mcp requests, headers are saved and body processing is requested.
//  2. RequestBody — parses the MCP JSON-RPC payload to extract tool_name and
//     purchase_amount, then calls PingAuthorize with the full attribute set.
func (s *PingAuthzShim) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	var savedHeaders map[string]string
	var traceID string

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("ext_proc stream error: %v", err)
			return err
		}

		// Phase 1: Request Headers
		if reqHeaders := msg.GetRequestHeaders(); reqHeaders != nil {
			traceID = fmt.Sprintf("req-%08x", time.Now().UnixNano()&0xFFFFFFFF)
			savedHeaders = flattenEnvoyHeaders(reqHeaders)
			resp := s.evaluateHeaders(traceID, savedHeaders)
			if err := stream.Send(resp); err != nil {
				return err
			}
			continue
		}

		// Phase 2: Request Body (only reached for authenticated /mcp requests)
		if reqBody := msg.GetRequestBody(); reqBody != nil {
			resp := s.evaluateBody(traceID, savedHeaders, reqBody.GetBody())
			if err := stream.Send(resp); err != nil {
				return err
			}
			continue
		}
	}
}

// evaluateHeaders handles fast-path decisions and requests body processing for /mcp.
func (s *PingAuthzShim) evaluateHeaders(traceID string, headers map[string]string) *extproc.ProcessingResponse {
	path := headers[":path"]

	// Discovery endpoints are always allowed — agents need them to find the authorization server.
	if strings.HasPrefix(path, "/.well-known") {
		log.Printf("[%s] passthrough path=%q", traceID, path)
		return buildPassthroughResponse()
	}

	if path != "/mcp" {
		return buildRejectResponse(
			typev3.StatusCode_NotFound, "", "",
			`{"error":"not_found","error_description":"Unknown path"}`,
		)
	}

	// No bearer token → 401 so the agent can discover and start the OAuth flow.
	if ExtractBearerToken(headers["authorization"]) == "" {
		log.Printf("[%s] no bearer token, returning 401", traceID)
		return buildRejectResponse(
			typev3.StatusCode_Unauthorized, s.getWWWAuthenticate(), "",
			`{"error":"unauthorized","error_description":"Bearer token required"}`,
		)
	}

	// Token present on /mcp — request the body so we can extract tool name + arguments.
	log.Printf("[%s] authenticated request to /mcp, requesting body", traceID)
	return buildRequestBodyResponse()
}

// mcpJsonRpcRequest represents the relevant fields of an MCP JSON-RPC request.
type mcpJsonRpcRequest struct {
	Method string `json:"method"`
	Params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"params"`
}

// evaluateBody parses the MCP JSON-RPC body, builds the policy attribute set,
// then calls PingAuthorize for an allow/deny decision.
func (s *PingAuthzShim) evaluateBody(traceID string, headers map[string]string, body []byte) *extproc.ProcessingResponse {
	attributes := buildPolicyAttributes(headers, body)
	logPolicyAttributes(traceID, attributes)

	allowed, decision, err := s.callPingAuthorize(pingAuthorizeRequest{Attributes: attributes})
	if err != nil {
		log.Printf("[%s] policy call failed: %v", traceID, err)
		return buildRejectBodyResponse(
			typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
			fmt.Sprintf(`{"error":"access_denied","decision_id":"%s","source":"ping-authorize"}`, traceID),
		)
	}

	toolName := attributes["mcp_tool_name"]
	log.Printf("[%s] ← PingAuthorize decision=%s tool=%q", traceID, decision.Decision, toolName)

	if allowed {
		return buildAllowBodyResponse(traceID)
	}

	reason := "Access denied by policy"
	if len(decision.Statements) > 0 && decision.Statements[0].Payload != "" {
		reason = decision.Statements[0].Payload
	}
	return buildRejectBodyResponse(
		typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
		fmt.Sprintf(`{"error":"access_denied","reason":"%s","decision":"deny","decision_id":"%s","source":"ping-authorize"}`, reason, traceID),
	)
}

// buildPolicyAttributes combines request headers and the MCP JSON-RPC body
// into a flat attribute map for PingAuthorize policy evaluation.
func buildPolicyAttributes(headers map[string]string, body []byte) map[string]string {
	attrs := make(map[string]string, len(headers)+6)
	for k, v := range headers {
		attrs[k] = v
	}

	// Replace raw authorization header with just the bearer token.
	token := ExtractBearerToken(attrs["authorization"])
	delete(attrs, "authorization")
	if token != "" {
		attrs["access_token"] = token
	}

	// Parse MCP JSON-RPC payload for tool name and arguments.
	var rpc mcpJsonRpcRequest
	if err := json.Unmarshal(body, &rpc); err != nil {
		return attrs
	}
	attrs["mcp_method"] = rpc.Method

	if rpc.Method != "tools/call" {
		return attrs
	}
	attrs["mcp_tool_name"] = rpc.Params.Name

	// Extract purchase-specific arguments for payment policy decisions.
	if rpc.Params.Name == "create_stripe_payment_intent" {
		for _, key := range []string{"product_id", "quantity", "total_price", "currency"} {
			if val, ok := rpc.Params.Arguments[key]; ok {
				attrs["mcp_"+key] = fmt.Sprintf("%v", val)
			}
		}
	}

	return attrs
}

// logPolicyAttributes logs the attributes being sent to PingAuthorize.
func logPolicyAttributes(traceID string, attrs map[string]string) {
	log.Printf("[%s] → PingAuthorize attributes:", traceID)
	log.Printf("[%s]   access_token=%s", traceID, attrs["access_token"])
	log.Printf("[%s]   mcp_method=%s mcp_tool_name=%s", traceID, attrs["mcp_method"], attrs["mcp_tool_name"])
	for _, key := range []string{"mcp_product_id", "mcp_quantity", "mcp_total_price", "mcp_currency"} {
		if v, ok := attrs[key]; ok {
			log.Printf("[%s]   %s=%s", traceID, key, v)
		}
	}
}

// getWWWAuthenticate returns the WWW-Authenticate header value for reject responses.
func (s *PingAuthzShim) getWWWAuthenticate() string {
	return fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource", scope="%s"`, s.mcpServerURL, s.mcpRequiredScopes)
}
