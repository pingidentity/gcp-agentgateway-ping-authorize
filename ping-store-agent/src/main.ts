import "dotenv/config";
import { app } from "./server.js";

const port = parseInt(process.env.PORT ?? "8080", 10);
app.listen(port, () => {
  console.log(`ping-store-agent listening on port ${port}`);
});
