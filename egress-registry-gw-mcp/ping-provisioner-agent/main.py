"""FastAPI entrypoint for the ping-provisioner-agent.

Exposes two endpoints:
  POST /provision  { "instruction": "..." }
      Creates an ADK agent (with MCP toolsets resolved from Agent Registry),
      runs the instruction, and returns the agent's final response.
  GET  /health
      Liveness probe — returns 200 OK.
"""
import logging

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from google.adk.runners import Runner
from google.adk.sessions import InMemorySessionService
from google.genai import types
from pydantic import BaseModel

from agent import create_agent
from config import settings

# Load and validate all required env vars at import time so the container
# fails fast on misconfiguration instead of crashing on the first request.
settings.load()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)

app = FastAPI(title="ping-provisioner-agent", version="1.0.0")


class ProvisionRequest(BaseModel):
    instruction: str


class ProvisionResponse(BaseModel):
    result: str


@app.get("/health")
async def health() -> JSONResponse:
    return JSONResponse({"status": "ok"})


@app.post("/provision", response_model=ProvisionResponse)
async def provision(req: ProvisionRequest) -> ProvisionResponse:
    """Run the provisioner agent for the given instruction."""
    if not req.instruction.strip():
        raise HTTPException(status_code=400, detail="instruction must not be empty")

    logger.info("provision request: %s", req.instruction[:120])

    try:
        agent = await create_agent()
    except Exception as exc:
        logger.error("Failed to create agent: %s", exc)
        raise HTTPException(status_code=500, detail=f"Agent creation failed: {exc}") from exc

    try:
        result = await _run_agent(agent, req.instruction)
    except Exception as exc:
        logger.error("Agent execution failed: %s", exc)
        raise HTTPException(status_code=500, detail=f"Agent execution failed: {exc}") from exc

    logger.info("provision result: %s", result[:200])
    return ProvisionResponse(result=result)


async def _run_agent(agent, instruction: str) -> str:
    """Run the ADK agent for a single instruction and return the final response text."""
    app_name = "ping-provisioner"
    user_id = "provisioner-service"

    session_service = InMemorySessionService()
    runner = Runner(
        agent=agent,
        app_name=app_name,
        session_service=session_service,
    )

    session = await session_service.create_session(
        app_name=app_name,
        user_id=user_id,
    )

    user_message = types.Content(
        parts=[types.Part(text=instruction)],
        role="user",
    )

    final_text = ""
    async for event in runner.run_async(
        user_id=user_id,
        session_id=session.id,
        new_message=user_message,
    ):
        if event.is_final_response() and event.content and event.content.parts:
            final_text = "".join(
                part.text for part in event.content.parts if part.text
            )

    return final_text or "(no response from agent)"


if __name__ == "__main__":
    port = settings.agent_port
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=port,
        log_level="info",
    )
