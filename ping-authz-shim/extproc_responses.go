package main

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

// ---- Response builders ----

// buildPassthroughResponse tells Envoy to forward the request unchanged.
// Used for public OAuth paths that don't require a token.
func buildPassthroughResponse() *extproc.ProcessingResponse {
	return continueResponse(nil)
}

// buildAllowResponse tells Envoy to forward the request to the downstream MCP
// server, adding tracing headers that record the PingAuthorize decision.
func buildAllowResponse(traceID string) *extproc.ProcessingResponse {
	return continueResponse(decisionHeaders("permit", traceID))
}

// buildRejectResponse short-circuits the request with the given status code and
// includes WWW-Authenticate so agents can discover or refresh their OAuth token.
func buildRejectResponse(code typev3.StatusCode, wwwAuth, traceID, body string) *extproc.ProcessingResponse {
	responseHeaders := []*corev3.HeaderValueOption{
		envoyHeader("content-type", "application/json"),
	}
	if wwwAuth != "" {
		responseHeaders = append(responseHeaders, envoyHeader("www-authenticate", wwwAuth))
	}
	if traceID != "" {
		for _, h := range decisionHeaders("deny", traceID).SetHeaders {
			responseHeaders = append(responseHeaders, h)
		}
	}
	return immediateResponse(code, responseHeaders, body)
}

// ---- Envoy wire-format helpers ----
// Envoy ext_proc responses have deeply nested protobuf structures. These helpers
// wrap that nesting so the response builders above stay readable.

// continueResponse tells Envoy to forward the request, optionally mutating
// headers first. Pass nil for no header mutations.
func continueResponse(mutation *extproc.HeaderMutation) *extproc.ProcessingResponse {
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extproc.HeadersResponse{
				Response: &extproc.CommonResponse{
					Status:         extproc.CommonResponse_CONTINUE,
					HeaderMutation: mutation,
				},
			},
		},
	}
}

// immediateResponse tells Envoy to short-circuit the request and return this
// status, headers, and body directly to the client.
func immediateResponse(code typev3.StatusCode, headers []*corev3.HeaderValueOption, body string) *extproc.ProcessingResponse {
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extproc.ImmediateResponse{
				Status:  &typev3.HttpStatus{Code: code},
				Headers: &extproc.HeaderMutation{SetHeaders: headers},
				Body:    []byte(body),
			},
		},
	}
}

// decisionHeaders returns a HeaderMutation with tracing headers that record
// the PingAuthorize decision outcome for observability.
func decisionHeaders(decision, traceID string) *extproc.HeaderMutation {
	return &extproc.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			envoyHeader("x-ping-authorize-decision", decision),
			envoyHeader("x-ping-authorize-decision-id", traceID),
		},
	}
}
