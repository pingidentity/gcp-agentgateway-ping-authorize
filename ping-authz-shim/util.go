package main

import (
	"crypto/tls"
	"net/http"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

// ---- Token helpers ----

// ExtractBearerToken returns the token from an "Authorization: Bearer <token>"
// header. Returns empty string if the header is missing or not Bearer-prefixed.
func ExtractBearerToken(authHeader string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return authHeader[len(prefix):]
	}
	return ""
}

// ---- HTTP helpers ----

// newTLSTransport returns an HTTP transport with TLS certificate verification
// optionally disabled. Only set skipVerify to true in development environments.
func newTLSTransport(skipVerify bool) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}
}

// ---- Envoy header helpers ----

// envoyHeader creates a single Envoy header key/value option using RawValue
// so GCLB forwards the value correctly (the string Value field is ignored by GCLB).
func envoyHeader(key, value string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{Key: key, RawValue: []byte(value)},
	}
}

// flattenEnvoyHeaders converts Envoy's protobuf header list into a plain
// key/value map for easy iteration and attribute passing.
func flattenEnvoyHeaders(headers *extproc.HttpHeaders) map[string]string {
	out := map[string]string{}
	if headers == nil || headers.GetHeaders() == nil {
		return out
	}
	for _, h := range headers.GetHeaders().GetHeaders() {
		key := strings.TrimSpace(h.GetKey())
		if key == "" {
			continue
		}
		out[key] = resolveHeaderValue(h)
	}
	return out
}

// resolveHeaderValue reads the value from an Envoy header. Envoy can send
// values as a UTF-8 string or raw bytes — this handles both.
func resolveHeaderValue(h *corev3.HeaderValue) string {
	if h == nil {
		return ""
	}
	if v := h.GetValue(); v != "" {
		return v
	}
	return string(h.GetRawValue())
}
