import "dotenv/config";
import express from "express";
import cors from "cors";
import { validateSubjectToken } from "./auth.js";
import { getOrCreateAgentSession, saveAgentConversation } from "./agent.js";
import type { ChatRequest } from "./util.js";
import { extractBearerToken, extractAgentResponseText, HttpError } from "./util.js";
import { AGENT_PORT, CORS_ORIGIN_CHAT_UI_STOREFRONT } from "./config.js";

const app = express();
app.use(cors({ origin: CORS_ORIGIN_CHAT_UI_STOREFRONT }));
app.use(express.json());

/** Request logging middleware. */
app.use((req, _res, next) => {
  console.log(`${req.method} ${req.path}`);
  next();
});

/** GET /health — Liveness check endpoint. */
app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

/** POST /chat — Validates the subject token, resolves an agent session, invokes the LLM, and returns the response. */
app.post("/chat", async (req, res) => {
  try {
    const subjectToken = extractBearerToken(req);
    const userId = await validateSubjectToken(subjectToken);
    const { message } = req.body as ChatRequest;
    if (!message) {
      res.status(400).json({ error: "Missing 'message' in request body" });
      return;
    }

    const agent = await getOrCreateAgentSession(userId, subjectToken);
    const result = await agent.invoke(message);
    saveAgentConversation(userId, agent.messages);
    res.json({ response: extractAgentResponseText(result) });
  } catch (err) {
    const statusCode = err instanceof HttpError ? err.statusCode : 500;
    const message = err instanceof Error ? err.message : "Internal server error";
    if (statusCode >= 500) console.error("Chat error:", err);
    res.status(statusCode).json({ error: message });
  }
});

app.listen(AGENT_PORT, () => {
  console.log(`ping-store-agent listening on port ${AGENT_PORT}`);
});
