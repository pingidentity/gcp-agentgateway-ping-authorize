# ping-authz-shim

gRPC service implementing Envoy's [ext_proc](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) protocol. The regional load balancer intercepts every inbound request and forwards it here for an authorization decision from [PingAuthorize](https://docs.pingidentity.com/pingauthorize/11.0/pingauthorize_policy_administration_guide/paz_policy_management.html) before the request reaches the downstream MCP server.

```
AI Agent → Regional Load Balancer → ping-authz-shim → PingAuthorize → allow / deny
```

## Request Phases

The shim intercepts requests in two phases:

1. **RequestHeaders** — fast-path decisions: passthrough for `/.well-known/*`, 401 for missing tokens, 404 for unknown paths. Authenticated `/mcp` requests proceed to phase 2.
2. **RequestBody** — parses the MCP JSON-RPC payload to extract tool name and arguments (e.g. product ID, purchase amount), then calls PingAuthorize with the full attribute set for a policy decision.

## Configuration

| Variable | Description |
|---|---|
| `SHIM_SERVER_PORT` | gRPC server port (set to `8080` for Cloud Run) |
| `PING_AUTHORIZE_URL` | PingAuthorize governance engine endpoint |
| `MCP_SERVER_URL` | Downstream MCP server URL (used in `WWW-Authenticate` for OAuth discovery) |
| `MCP_REQUIRED_SCOPES` | Space-separated scopes advertised in `WWW-Authenticate` headers |
| `PING_AUTHORIZE_SKIP_TLS_VERIFY` | Set to `true` to disable TLS cert verification for PingAuthorize calls (dev only) |

## Deployment

Deployed to Cloud Run via the Cloud Build pipeline at [`../deploy/gcp/cloudbuild.ping-authz-shim.yaml`](../deploy/gcp/cloudbuild.ping-authz-shim.yaml).

**Trigger from repo root:**

```bash
gcloud builds submit \
  --config ingress-public-lb-mcp/deploy/gcp/cloudbuild.ping-authz-shim.yaml \
  --substitutions \
    _PING_AUTHORIZE_URL=https://your-ping-authorize.com/governance-engine,\
    _MCP_SERVER_URL=https://your-mcp-lb.com,\
    _MCP_REQUIRED_SCOPES="openid profile email stripe_mcp:invoke"
```

The pipeline builds the Docker image, pushes it to Artifact Registry, and deploys to Cloud Run with `--ingress internal-and-cloud-load-balancing` and `--use-http2` (required for gRPC).

Copy `.env.sample` to `.env` and fill in values for local development.

## Files

| File | Responsibility |
|---|---|
| `main.go` | Entrypoint — loads config, wires gRPC server, starts listener |
| `extproc_handler.go` | Stream handler, header/body evaluation, policy attribute building |
| `extproc_responses.go` | Envoy response builders (allow, reject, passthrough) |
| `ping_authorize_client.go` | HTTP client for PingAuthorize governance engine decisions |
| `util.go` | Shared helpers — token extraction, header parsing, TLS config |
