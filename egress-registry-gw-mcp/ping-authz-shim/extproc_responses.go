package main

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprochttp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

// ---- Headers-phase response builders ----

// buildPassthroughResponse tells Envoy to forward the request unchanged.
func buildPassthroughResponse() *extproc.ProcessingResponse {
	return headersResponse(extproc.CommonResponse_CONTINUE, nil)
}

// buildRequestBodyResponse tells Envoy to continue but also send us the request body.
// Uses mode_override to enable body processing for this specific request.
func buildRequestBodyResponse() *extproc.ProcessingResponse {
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extproc.HeadersResponse{
				Response: &extproc.CommonResponse{
					Status: extproc.CommonResponse_CONTINUE,
				},
			},
		},
		ModeOverride: &extprochttp.ProcessingMode{
			RequestBodyMode: extprochttp.ProcessingMode_BUFFERED,
		},
	}
}

// buildRejectResponse short-circuits during the headers phase.
func buildRejectResponse(code typev3.StatusCode, wwwAuth, traceID, body string) *extproc.ProcessingResponse {
	return immediateResponse(code, buildResponseHeaders(wwwAuth, traceID, "deny"), body)
}

// ---- Body-phase response builders ----

// buildAllowBodyResponse tells Envoy to forward the request after body inspection,
// adding tracing headers that record the PingAuthorize decision.
func buildAllowBodyResponse(traceID string) *extproc.ProcessingResponse {
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_RequestBody{
			RequestBody: &extproc.BodyResponse{
				Response: &extproc.CommonResponse{
					Status:         extproc.CommonResponse_CONTINUE,
					HeaderMutation: decisionHeaders("permit", traceID),
				},
			},
		},
	}
}

// buildRejectBodyResponse short-circuits during the body phase.
func buildRejectBodyResponse(code typev3.StatusCode, wwwAuth, traceID, body string) *extproc.ProcessingResponse {
	return immediateResponse(code, buildResponseHeaders(wwwAuth, traceID, "deny"), body)
}

// ---- Shared helpers ----

func buildResponseHeaders(wwwAuth, traceID, decision string) []*corev3.HeaderValueOption {
	headers := []*corev3.HeaderValueOption{
		envoyHeader("content-type", "application/json"),
	}
	if wwwAuth != "" {
		headers = append(headers, envoyHeader("www-authenticate", wwwAuth))
	}
	if traceID != "" {
		for _, h := range decisionHeaders(decision, traceID).SetHeaders {
			headers = append(headers, h)
		}
	}
	return headers
}

func headersResponse(status extproc.CommonResponse_ResponseStatus, mutation *extproc.HeaderMutation) *extproc.ProcessingResponse {
	return &extproc.ProcessingResponse{
		Response: &extproc.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extproc.HeadersResponse{
				Response: &extproc.CommonResponse{
					Status:         status,
					HeaderMutation: mutation,
				},
			},
		},
	}
}

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

func decisionHeaders(decision, traceID string) *extproc.HeaderMutation {
	return &extproc.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			envoyHeader("x-ping-authorize-decision", decision),
			envoyHeader("x-ping-authorize-decision-id", traceID),
		},
	}
}
