// This service implements Envoy's ExternalProcessor gRPC interface, allowing
// Google Cloud Load Balancer / Agent Gateway Traffic Extensions to intercept
// HTTP requests and forward them here for an authorization decision from
// PingAuthorize. Allowed requests are passed through to the downstream
// MCP server; denied requests receive an immediate HTTP error response.
package main

import (
	"log"
	"net"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(0)
	shimPort := requireEnv("SHIM_SERVER_PORT")
	pingAuthorizeURL := requireEnv("PING_AUTHORIZE_URL")
	pingAuthorizeSkipTLS := requireEnv("PING_AUTHORIZE_SKIP_TLS_VERIFY") == "true"
	mcpServerURL := requireEnv("MCP_SERVER_URL")
	mcpRequiredScopes := requireEnv("MCP_REQUIRED_SCOPES")

	// Build the gRPC server and register the ext_proc authorization shim.
	// The Agent Gateway connects here via gRPC (Envoy ext_proc protocol) and
	// calls Process() for each intercepted request. The shim consults
	// PingAuthorize and returns an allow/deny decision.
	grpcServer := grpc.NewServer()
	extproc.RegisterExternalProcessorServer(grpcServer, NewPingAuthzShim(pingAuthorizeURL, mcpServerURL, mcpRequiredScopes, pingAuthorizeSkipTLS))

	// Start listening — gRPC runs over HTTP/2 on a TCP port.
	lis, err := net.Listen("tcp", ":"+shimPort)
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", shimPort, err)
	}
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
