package main

import (
	"encoding/json"
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
// every intercepted request. It inspects both headers and body to extract policy attributes
// (bearer token, MCP tool name, purchase amount) and forwards them to PingAuthorize for an
// allow/deny decision before the request reaches the downstream MCP server.
type PingAuthzShim struct {
	httpClient       *http.Client
	pingAuthorizeURL string
	mcpServerURL     string
	scopes           string
}

// NewPingAuthzShim initializes the ext_proc shim from environment configuration.
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

// Process handles the bidirectional gRPC stream between Envoy and this ext_proc shim.
//
// Two phases are intercepted:
//  1. RequestHeaders — fast-path decisions (passthrough, 401, 404). For authenticated
//     /mcp requests, headers are saved and body processing is requested.
//  2. RequestBody — parses the MCP JSON-RPC payload to extract tool_name and
//     purchase_amount, then calls PingAuthorize with the full attribute set.
func (s *PingAuthzShim) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	// Per-stream state: headers from phase 1 are used in phase 2.
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
			traceID = fmt.Sprintf("pa-%d", time.Now().UnixNano())
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

// evaluateBody parses the MCP JSON-RPC body, extracts tool_name and purchase_amount,
// then calls PingAuthorize with the complete attribute set.
func (s *PingAuthzShim) evaluateBody(traceID string, headers map[string]string, body []byte) *extproc.ProcessingResponse {
	// Start with all request headers as attributes.
	attributes := make(map[string]string, len(headers)+3)
	for k, v := range headers {
		attributes[k] = v
	}

	// Parse the MCP JSON-RPC payload and extract tool name + arguments for policy evaluation.
	var rpc mcpJsonRpcRequest
	if err := json.Unmarshal(body, &rpc); err == nil {
		attributes["mcp_method"] = rpc.Method

		if rpc.Method == "tools/call" {
			attributes["mcp_tool_name"] = rpc.Params.Name

			// Only extract purchase args for the payment intent tool.
			if rpc.Params.Name == "create_stripe_payment_intent" {
				if productID, ok := rpc.Params.Arguments["product_id"]; ok {
					attributes["mcp_product_id"] = fmt.Sprintf("%v", productID)
				}
				if quantity, ok := rpc.Params.Arguments["quantity"]; ok {
					attributes["mcp_purchase_quantity"] = fmt.Sprintf("%v", quantity)
				}
				if totalPrice, ok := rpc.Params.Arguments["total_price"]; ok {
					attributes["mcp_total_price"] = fmt.Sprintf("%v", totalPrice)
				}
				if currency, ok := rpc.Params.Arguments["currency"]; ok {
					attributes["mcp_currency"] = fmt.Sprintf("%v", currency)
				}
			}
		}
	}

	// Replace the raw authorization header with just the bearer token.
	token := ExtractBearerToken(attributes["authorization"])
	delete(attributes, "authorization")
	if token != "" {
		attributes["access_token"] = token
	}

	// Log the attributes being sent to PingAuthorize for observability.
	mcpMethod := valueOrNA(attributes["mcp_method"])
	toolName := valueOrNA(attributes["mcp_tool_name"])
	log.Printf("[%s] → PingAuthorize attributes:", traceID)
	log.Printf("[%s]   access_token=%s", traceID, token)
	log.Printf("[%s]   mcp_method=%s mcp_tool_name=%s", traceID, mcpMethod, toolName)
	// Log purchase-specific attributes only when present.
	if v, ok := attributes["mcp_product_id"]; ok {
		log.Printf("[%s]   mcp_product_id=%s", traceID, v)
	}
	if v, ok := attributes["mcp_purchase_quantity"]; ok {
		log.Printf("[%s]   mcp_purchase_quantity=%s", traceID, v)
	}
	if v, ok := attributes["mcp_total_price"]; ok {
		log.Printf("[%s]   mcp_total_price=%s", traceID, v)
	}
	if v, ok := attributes["mcp_currency"]; ok {
		log.Printf("[%s]   mcp_currency=%s", traceID, v)
	}

	// Call PingAuthorize with all attributes for a policy decision.
	req := pingAuthorizeRequest{Attributes: attributes}
	allowed, decision, err := s.callPingAuthorize(req)
	if err != nil {
		log.Printf("[%s] policy call failed: %v", traceID, err)
		return buildRejectBodyResponse(
			typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
			`{"error":"access_denied","decision_id":"`+traceID+`","source":"ping-authorize"}`,
		)
	}
	log.Printf("[%s] ← PingAuthorize decision=%s tool=%q", traceID, decision.Decision, toolName)

	if allowed {
		return buildAllowBodyResponse(traceID)
	}
	return buildRejectBodyResponse(
		typev3.StatusCode_Forbidden, s.getWWWAuthenticate(), traceID,
		`{"error":"access_denied","decision":"deny","decision_id":"`+traceID+`","source":"ping-authorize"}`,
	)
}

// getWWWAuthenticate returns the WWW-Authenticate header value for reject responses.
func (s *PingAuthzShim) getWWWAuthenticate() string {
	return fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource", scope="%s"`, s.mcpServerURL, s.scopes)
}
