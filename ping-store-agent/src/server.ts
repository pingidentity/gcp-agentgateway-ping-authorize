/**
 * Express Server
 *
 * POST /chat  — Validates the subject token, resolves an agent session,
 *               invokes the LLM, and returns the agent's text response.
 * GET /health — Liveness check.
 */

import express from "express";
import cors from "cors";
import { validateSubjectToken } from "./auth.js";
import { getOrCreateSession, saveConversation } from "./session.js";

interface ChatRequest {
  message: string;
}

export const app = express();
app.use(cors());
app.use(express.json());

app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

app.post("/chat", async (req, res) => {
  try {
    const authHeader = req.headers.authorization;
    if (!authHeader?.startsWith("Bearer ")) {
      res.status(401).json({ error: "Missing Bearer token" });
      return;
    }
    const subjectToken = authHeader.slice(7);

    validateSubjectToken(subjectToken);

    const { message } = req.body as ChatRequest;
    if (!message) {
      res.status(400).json({ error: "Missing 'message' in request body" });
      return;
    }

    const agent = await getOrCreateSession(subjectToken);
    const result = await agent.invoke(message);
    saveConversation(subjectToken, agent.messages);

    // Extract text from Strands agent result (may be string or content block array)
    let text: string;
    if (typeof result === "string") {
      text = result;
    } else if (result?.lastMessage?.content) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const blocks = result.lastMessage.content as any[];
      text = blocks.filter((b) => b.text).map((b) => b.text).join("\n");
    } else {
      text = JSON.stringify(result);
    }

    res.json({ response: text });
  } catch (err) {
    console.error("Chat error:", err);
    res.status(500).json({
      error: err instanceof Error ? err.message : "Internal server error",
    });
  }
});
