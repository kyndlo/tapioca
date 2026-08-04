import { llmsText } from "../llm/agent-content";

export function GET() {
  return new Response(llmsText, {
    headers: {
      "cache-control": "public, max-age=300",
      "content-type": "text/plain; charset=utf-8",
    },
  });
}
