# GCP Agent Gateway + PingAuthorize

A proof of concept demonstrating centralized authorization for [MCP](https://modelcontextprotocol.io/) servers on GCP, using [PingAuthorize](https://docs.pingidentity.com/pingauthorize) as the policy decision point enforced at the network edge via [GCP's Traffic Extensions](https://cloud.google.com/service-extensions/docs/callouts-overview) and the Envoy [ext_proc](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) protocol.

The sample MCP server wraps the [Stripe API](https://docs.stripe.com/api) and runs behind a GCP Agent Gateway. Every inbound request is intercepted by an ext_proc service that parses the MCP request body, extracts the tool name and arguments, and calls PingAuthorize for a policy decision — all **before** the request reaches the MCP server.

> [!WARNING]
> This is a proof of concept intended to demonstrate the integration pattern. **You must implement all preventive and defense-in-depth security measures before deploying to production.** Please review the [MCP Security Best Practices](https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices), the [Envoy External Processing filter documentation](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter), and the [GCP Service Extensions overview](https://cloud.google.com/service-extensions/docs/overview) before using this in any production environment.

## Agent Types

This project supports two distinct agent architectures, each with its own authentication and authorization flow. See the dedicated documentation for details on how each works:

| Architecture | Description | Documentation |
|---|---|---|
| **Delegated Agent** | A backend service acts on behalf of a user via [RFC 8693](https://datatracker.ietf.org/doc/html/rfc8693) token exchange with delegation. The delegated token carries both the user's identity and the agent's identity. | [→ Delegated Agent](_docs/delegated-agent.md) |
| **Attended Agent** | An MCP-aware client (e.g., Claude Desktop) connects directly to the Agent Gateway, handling OAuth discovery and token acquisition itself. | [→ Attended Agent](_docs/attended-agent.md) |

Both architectures share the same authorization enforcement point — every request passes through `ping-authz-shim`, which consults PingAuthorize before allowing access to the MCP server.

## Services

| Service | Description |
|---|---|
| [`ping-authz-shim`](./ping-authz-shim/) | Envoy ext_proc shim — intercepts every request, parses MCP payloads, and consults PingAuthorize for a policy decision |
| [`stripe-mcp`](./stripe-mcp/) | MCP server exposing Stripe tools (product catalog, customer lookup, payments) |
| [`ping-store-agent`](./ping-store-agent/) | Delegated agent backend — handles user sessions, token exchange, and MCP tool invocation |
| [`ping-chat-ui-storefront`](./ping-chat-ui-storefront/) | Chat UI front-end — user authentication and conversation interface |

## Repository Structure

```
├── ping-authz-shim/            # ext_proc authorization shim (Go, gRPC)
├── stripe-mcp/                 # MCP server with Stripe tools (Go, HTTP)
├── ping-store-agent/           # Delegated agent backend (TypeScript, Express)
├── ping-chat-ui-storefront/    # Chat UI front-end (React, Vite)
├── _docs/                      # Architecture documentation
└── deploy/gcp/                 # Cloud Build configs
```

## Protocols & Standards

- [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) — tool interface between agent and server
- [Envoy ext_proc](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) — external processing filter for request-level auth decisions
- [OAuth 2.0 Token Exchange (RFC 8693)](https://datatracker.ietf.org/doc/html/rfc8693) — delegation token exchange for agent-on-behalf-of-user
- [OAuth 2.0 Protected Resource Metadata (RFC 9728)](https://datatracker.ietf.org/doc/html/rfc9728) — resource server discovery
- [OAuth 2.0 Authorization Server Metadata (RFC 8414)](https://datatracker.ietf.org/doc/html/rfc8414) — authorization server discovery
- [OAuth 2.0 Dynamic Client Registration (RFC 7591)](https://datatracker.ietf.org/doc/html/rfc7591) — agent self-registration
- [PKCE (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636) — proof key for code exchange

## Prerequisites

- **GCP project** with Cloud Run, Cloud Load Balancing, and Service Extensions APIs enabled
- **Stripe account** with a [secret API key](https://docs.stripe.com/keys) and at least one customer with a payment method on file — the customer's email must match the user's email in PingOne AIC
- **PingOne AIC tenant** configured as the OAuth 2.0 authorization server with Dynamic Client Registration enabled, token exchange grants, and a `may_act` script for delegation
- **PingAuthorize** deployed on a GCE VM in the same GCP project, behind a GCP load balancer with a Google-managed SSL certificate

## Deployment

Both core services run on **Cloud Run** behind a **GCP Agent Gateway** with Traffic Extensions enabled.

1. Deploy `ping-authz-shim` and `stripe-mcp` to Cloud Run
2. Create serverless NEGs and backend services for each
3. Provision the Agent Gateway with a URL map, SSL certificate, and forwarding rule
4. Create a Traffic Extension callout pointing to the shim's backend service with request body processing enabled
5. Attach the Traffic Extension to the Agent Gateway's URL map

The delegated agent services (`ping-store-agent` and `ping-chat-ui-storefront`) run on separate infrastructure and communicate with the Agent Gateway over HTTPS.

All Cloud Run services are configured with `--ingress internal-and-cloud-load-balancing` so they are only reachable through the Agent Gateway — direct requests from the public internet are blocked.
