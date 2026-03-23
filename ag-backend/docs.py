"""
ag-backend — Genie AI Multi-Agent Financial Analysis Backend
=============================================================

Overview
--------
``ag-backend`` is the core backend for the **Genie AI** smart financial assistant.
It implements a multi-agent workflow built on top of `LangGraph
<https://github.com/langchain-ai/langgraph>`_, where specialised agents
collaborate to analyse a user's bank transactions and surface actionable
financial intelligence.

The workflow is driven by an entry-point script (``main.py``) and is composed
of the following layers:

.. code-block:: text

    main.py
       └── core.graph.build_graph()          # Compiles the LangGraph DAG
             ├── core.orchestrator            # Central planner & lifecycle manager
             ├── core.classifier              # 3-tier intent router (NLU → SLM → LLM)
             ├── agents.local.supervisor      # Fan-out to local specialist agents
             │     ├── agents.local.spending_analysis
             │     ├── agents.local.anomaly_detection
             │     └── agents.local.cash_flow_forecasting
             ├── agents.local.reasoning_llm   # Report synthesis
             └── agents.remote.remote_client  # External/federated agent stub

Package Structure
-----------------
::

    ag-backend/
    ├── main.py               Entry point; loads CSV data and runs the graph
    ├── core/                 Orchestration, classification, graph construction
    ├── agents/
    │   ├── local/            On-device specialist agent nodes
    │   └── remote/           Remote/federated agent client
    ├── integration/          MCP protocol gateway
    ├── knowledge/            Vector DB / RAG knowledge layer
    ├── storage/              Shared LangGraph state schema
    ├── telemetry/            Observability / tracing utilities
    ├── data/                 Sample transaction CSV files
    └── scripts/              Data-generation helper scripts

Quick Start
-----------
.. code-block:: bash

    # Install dependencies
    pip install -r requirements.txt

    # Run with the default sample dataset
    python main.py

    # Run with a custom CSV file
    python main.py path/to/transactions.csv

Expected CSV columns: ``date``, ``description``, ``amount``, ``category``

Key Design Principles
---------------------
* **Federated agents** – local and remote agents are decoupled via the
  registry and MCP protocol, making it straightforward to add new agents.
* **Tiered intent classification** – cheap regex (NLU) runs first; only
  escalates to weighted scoring (SLM) or semantic fallback (LLM) when needed.
* **Observability-first** – every agent node is wrapped with the
  ``@trace_execution`` decorator for correlation-ID-based audit logging.
* **Graceful degradation** – forecasting and anomaly detection degrade
  safely when data is too sparse, and the report synthesiser uses a
  template fallback when no LLM API key is present.
"""
