# ping-authz-shim

Envoy [ext_proc](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter) shim that sits between GCLB and your MCP server. Every request gets its headers evaluated by [PingAuthorize](https://docs.pingidentity.com/pingauthorize/11.0/pingauthorize_policy_administration_guide/paz_policy_management.html) before it ever reaches the downstream MCP server (if permitted to do so).

```
Client → GCLB Traffic Extension → ping-authz-shim → PingAuthorize → allow / deny
```

## Configuration

| Variable | Description |
|---|---|
| `PING_AUTHORIZE_URL` | PingAuthorize governance engine endpoint |
| `MCP_SERVER_URL` | Downstream MCP server URL (used in `WWW-Authenticate` for OAuth discovery) |
| `OAUTH_SCOPES` | Space-delimited scopes advertised in `WWW-Authenticate` headers |
| `SKIP_TLS_VERIFY` | Set to `"true"` to disable TLS cert verification (dev only) |

## Files

| File | Responsibility |
|---|---|
| `main.go` | gRPC server bootstrap |
| `extproc_handler.go` | Stream handler + request evaluation logic |
| `extproc_responses.go` | Envoy response builders (allow, reject, passthrough) |
| `ping_authorize_client.go` | HTTP client for PingAuthorize decisions |
| `util.go` | Shared helpers (token extraction, header parsing, TLS, path matching) |
