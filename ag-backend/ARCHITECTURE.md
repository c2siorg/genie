# Multi-Agent System Architecture

This document outlines the design principles, architectural topology, and directory structure of the `genie_dummy` project - a Financial Transaction Analyst built on a Modular Monolith multi-agent framework.

## 1. Architectural Topologies & Paradigms

The current deployment follows a **Modular Monolith** pattern. While the orchestrator and specialized agents reside in a single codebase (allowing shared infrastructure and zero-latency communication), they are logically decoupled as independent modules with strict layer boundaries.

The core rule of our architecture is **Outer layers depend on inner ones**, governed by a central Orchestrator. Specialized agents **never** communicate directly with external tools or each other without prior mediation through secure, explicitly tracked channels (e.g., the Supervisor or Orchestrator).

## 2. Directory Structure & Role Breakdown

The codebase is organized into seven distinct, separated layers.

### `/` (Root directory)
*   **`main.py`** 
    *   **Layer:** User Application
    *   **Role:** The frontend interface entry point. It handles data ingestion, redaction of Personally Identifiable Information (PII) before state initialization, semantic cache hits, and initiates the multi-agent pipeline over a structured `SystemState`.

---

### `core/`
*   **Layer:** Orchestration Layer
*   **Role:** The central nervous system of the application. It receives requests, maintains context, handles errors, and dictates traffic routing.
*   **Files:**
    *   **`graph.py`**: Wires up the state graph using LangGraph, structurally defining the flow between core components.
    *   **`orchestrator.py`**: The Semantic Kernel. Checks the registry for agent availability, intercepts unresolvable intents (triggering an 'IDK' human-handoff), and maintains global workflow lifecycle.
    *   **`classifier.py`**: The "Semantic Router" utilizing a fallback pattern. It attempts cheap intent extraction (NLU Tier), falls back to statistical classification (SLM Tier), and relies on expensive reasoning (LLM Tier) only if confidence remains low.
    *   **`registry.py`**: A mocked database acting as a dynamic catalog of all agents, maintaining their specific capabilities, endpoints, and health status for the orchestrator to query.

---

### `agents/`
*   **Layer:** Agent Layer (Federated Model)
*   **Role:** The actual processing units, differentiated by their deployment location:
    *   **Local Agents**: Specialized experts running in the same process space as the orchestrator. They provide zero-latency execution and direct state access.
    *   **Remote Agents**: Agents accessible across network boundaries. They require formal communication protocols, handle network-specific error logic (retries, timeouts), and enforce independent authentication.
*   **Directories & Files:**
    *   **`local/supervisor.py`**: An intermediate coordinator. It breaks down complex tasks and simulates a **Parallel Fan-Out** distribution across all local domain experts before executing a **Chained Sequence** to synthesize the results.
    *   **`local/spending_analysis.py`, `local/anomaly_detection.py`, `local/cash_flow_forecasting.py`**: Specialized domain experts running locally to execute concurrent analyses.
    *   **`local/recommendation.py`**: A local synthesizer taking chained input from the financial domain experts.
    *   **`remote/remote_client.py`**: Stub representing a connection to a remote agent, demonstrating the network boundary abstraction.

---

### `storage/`
*   **Layer:** Storage Layer
*   **Role:** Persistent and isolated memory boundaries for short-term interactions and system states.
*   **Files:**
    *   **`state.py`**: Defines the `SystemState` `TypedDict`. Represents a segregated NoSQL/Document structure isolating Short-Term Memory (STM) conversation history (enforcing strict schema via metadata like `session_id`, `message_id`, and `source`) from active Registry tracking and Agent Runtime parameters.

---

### `integration/`
*   **Layer:** Integration Layer
*   **Role:** Standardized communication with external tools and APIS. Agents cannot hit external APIs directly.
*   **Files:**
    *   **`mcp_protocol.py`**: Implements the Model Context Protocol (MCP) Server logic. Acts as a strict Security Gateway. It explicitly enforces Authentication Checks (simulated OAuth/Bearer tokens) and Role-Based Access Control (RBAC) policies before permitting an external tool adapter execution.

---

### `knowledge/`
*   **Layer:** Knowledge Layer
*   **Role:** Domain-specific factual grounding and prompt optimization layers.
*   **Files:**
    *   **`vector_db.py`**: Implements RAG (Retrieval-Augmented Generation) patterns.
        *   Contains **Tool Vector Search**: Dynamically returns relevant tools required based on the user prompt to prevent blowing out the context window.
        *   Contains **Semantic Caching**: Checks prior prompts to skip repetitive data pipeline loads.

---

### `telemetry/`
*   **Layer:** Observability & Evaluation Layer
*   **Role:** Audit trails, tracking, and governance.
*   **Files:**
    *   **`observability.py`**: Traces system messaging. Enforces **Data Isolation** by explicitly redacting PII payloads prior to logging, and records unique `TraceIDs`, calling identities, and I/O hashes to fulfill auditing standards.

## 3. Communication Patterns Utilized

- **Mediated Sync/Async Request-Based Routing**: Orchestrator controls all node execution paths dynamically through `state.py` transitions.
- **Message-Driven Fan-Out**: The `supervisor.py` node initiates parallel state updates across three agents at once.
- **Message-Driven Chained Task Sequencing**: The `supervisor.py` aggregates those parallel results as an input sequence directly into the recommendation engine before finally closing the pipeline back up to the user loop.
