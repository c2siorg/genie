import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import "./styles/tokens.css";
import "./styles/shell.css";
import App from "./App";
import { SessionProvider } from "./auth/session";

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("root element #root not found");

// basename matches the Go embed mount (/ui/app/) so client routes resolve.
createRoot(rootEl).render(
  <StrictMode>
    <BrowserRouter basename="/ui/app">
      <SessionProvider>
        <App />
      </SessionProvider>
    </BrowserRouter>
  </StrictMode>,
);
