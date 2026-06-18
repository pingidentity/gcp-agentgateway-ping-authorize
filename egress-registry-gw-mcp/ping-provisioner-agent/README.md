# ping-provisioner-agent

FastAPI + ADK Python service that provisions user accounts across PingOne AIC
and Microsoft Entra. This service is deployed to **Cloud Run** via Cloud Build
(`cloudbuild.ping-provisioner-agent.yaml`) and registered in GCP Agent Registry.

> **Note:** The production deployment of this agent uses **Vertex AI Agent Runtime**
> via `deploy/gcp/deploy_agent.py` rather than this Cloud Run container directly.
> The Agent Runtime deployment enables Workload Identity Federation, RFC 8693
> token exchange, and AGENT_IDENTITY egress through the Agent Gateway.
> This folder contains the Cloud Run-compatible version of the agent code.

## Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/provision` | Run a provisioning instruction |
| `GET` | `/health` | Liveness probe |

```json
// POST /provision
{ "instruction": "Provision alice@example.com in both PingOne AIC and Entra" }

// Response
{ "result": "Provisioned alice@example.com: AIC user_id=abc123, Entra user_id=def456" }
```

## Environment Variables

```
GOOGLE_CLOUD_PROJECT=       # GCP project for Vertex AI / Gemini API
GOOGLE_CLOUD_LOCATION=us-central1
AGENT_PORT=3000
GEMINI_MODEL=gemini-2.5-flash
PINGONE_AIC_MCP_URL=        # https://gw-pingone-aic-mcp-*.run.app/mcp
ENTRA_MCP_URL=              # https://gw-entra-mcp-*.run.app/mcp
```

## Local Development

```bash
pip install -r requirements.txt
cp .env.sample .env
python main.py
```

## Deploy via Cloud Build

```bash
gcloud builds submit \
  --config egress-registry-gw-mcp/deploy/gcp/cloudbuild/ping-provisioner-agent.yaml .
```

For Agent Runtime deployment (production), see `deploy/gcp/deploy_agent.py`.
