// Package main is the entrypoint for the PingAuthorize ext_proc shim.
//
// This service implements Envoy's ExternalProcessor gRPC interface, allowing
// Google Cloud Load Balancer / Agent Gateway Traffic Extensions to intercept
// HTTP requests and forward them here for an authorization decision from
// PingAuthorize. Allowed requests are passed through to the downstream
// MCP server; denied requests receive an immediate HTTP error response.
package main

import (
	"fmt"
	"log"
	"net"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
)

// grpcPort is the fixed port this shim listens on. Cloud Run expects containers
// to bind on 8080 by default.
const grpcPort = 8080

func main() {
	// Open the TCP listener that the gRPC server will accept connections on.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen on port %d: %v", grpcPort, err)
	}

	// Create a new gRPC server instance.
	s := grpc.NewServer()

	// Instantiate the PingAuthorize ext_proc service, which handles
	// incoming RequestHeaders phases and issues allow/deny decisions.
	authzShim := NewPingAuthzShim()

	// Register the authz service against Envoy's ExternalProcessor interface.
	// GCLB Traffic Extensions will invoke Process() on this service for each
	// intercepted request.
	extproc.RegisterExternalProcessorServer(s, authzShim)

	log.Printf("PingAuthorize shim listening on port %d...", grpcPort)

	// Begin serving gRPC requests. Blocks until the server is stopped.
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
