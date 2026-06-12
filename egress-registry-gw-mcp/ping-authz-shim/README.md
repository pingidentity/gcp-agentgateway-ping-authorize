# ping-authz-shim (egress-registry-gw-mcp)

Envoy `ext_proc` authorization shim for the identity provisioning Agent Gateway.
Modified from the UC1 shim (`ingress-public-lb-mcp/ping-authz-shim`) with two
targeted changes for the provisioning use case.

## Changes from UC1

### 1. Accept `/mcp/*` paths

The UC1 shim only accepted requests to `/mcp`. This version accepts any path
matching `/mcp/pingone`, `/mcp/entra`, or any deeper sub-path — allowing the
internal LB to route to two different MCP backends while using a single shim.

```go
// Before (UC1)
if path != "/mcp" { ... reject ... }

// After (UC2)
if path != "/mcp" && !strings.HasPrefix(path, "/mcp/") { ... reject ... }
```

### 2. Provisioning policy attributes

The UC1 shim extracted Stripe purchase arguments (`product_id`, `quantity`,
`total_price`, `currency`). This version extracts identity provisioning
arguments for policy decisions:

```go
for _, key := range []string{"username", "email", "enabled"} {
    if val, ok := rpc.Params.Arguments[key]; ok {
        attrs["mcp_"+key] = fmt.Sprintf("%v", val)
    }
}
```

The `:path` header (already present) tells PingAuthorize which identity
system is being targeted (`/mcp/pingone` vs `/mcp/entra`).

## Policy Attributes

The full attribute map sent to PingAuthorize:

```json
{
  "attributes": {
    "access_token": "<agent bearer token>",
    ":path": "/mcp/entra",
    ":method": "POST",
    "mcp_method": "tools/call",
    "mcp_tool_name": "provision_user",
    "mcp_email": "alice@example.com",
    "mcp_username": "alice.smith"
  }
}
```

## Environment Variables

```
SHIM_SERVER_PORT=8080
PING_AUTHORIZE_URL=          # PingAuthorize governance engine endpoint
MCP_SERVER_URL=              # Agent Gateway URL (used in WWW-Authenticate)
PING_AUTHORIZE_SKIP_TLS_VERIFY=false
MCP_REQUIRED_SCOPES=pingone:provisioning
```

## All Other Files

`extproc_responses.go`, `ping_authorize_client.go`, `util.go`, `main.go`,
`go.mod`, and `Dockerfile` are identical to UC1. Only `extproc_handler.go`
differs.

## Local Development

```bash
cp .env.sample .env
export $(cat .env | xargs)
go run .
```

## Docker

```bash
docker build -t ping-authz-shim-egress .
docker run -p 8080:8080 --env-file .env ping-authz-shim-egress
```
