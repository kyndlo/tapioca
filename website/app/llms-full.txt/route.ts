import { llmsFullText } from "../llm/agent-content";

export function GET() {
  return new Response(llmsFullText, {
    headers: {
      "cache-control": "public, max-age=300",
      "content-type": "text/plain; charset=utf-8",
    },
  });
}
