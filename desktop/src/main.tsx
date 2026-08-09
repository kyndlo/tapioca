import { createRoot } from "react-dom/client";
import App from "./app/App";
import "./styles/tokens.css";
import "./styles/app.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Tapioca renderer root is missing");
}

// The renderer talks to a bounded local control service. React development
// StrictMode intentionally replays effects, which duplicates startup IPC and
// can exhaust that bounded service even though packaged builds run once.
createRoot(root).render(<App />);
