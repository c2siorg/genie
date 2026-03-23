"""
core — Orchestration, Classification & Graph Construction
=========================================================

The ``core`` package contains the central coordination logic for the Genie AI
multi-agent system.  It is responsible for:

1. **Building the LangGraph DAG** (``graph.py``) that connects all agent nodes.
2. **Orchestrating** the high-level workflow and error-recovery strategy
   (``orchestrator.py``).
3. **Classifying user intent** into the correct processing layer using a
   three-tier pipeline (``classifier.py``).
4. **Maintaining a dynamic agent registry** of available agents and their
   metadata (``registry.py``).

Modules
-------

graph.py — ``build_graph()``
    Constructs and compiles the ``StateGraph`` (LangGraph DAG).

    Workflow::

        START
          └─► orchestrator ─► classifier ─► [local_supervisor | remote_agent]
                                                      │
                                              local_supervisor
                                                      │
                                              reasoning_llm ─► END

    Conditional edges from ``classifier`` use the ``target_layer`` state field
    to route to either the local or remote agent branch.

orchestrator.py — ``orchestrator_node``
    The first node in the graph.  Responsibilities:

    * Refreshes the live agent registry via ``core.registry.get_available_agents()``.
    * Inspects accumulated ``errors`` in state and logs warnings for error-recovery.
    * Propagates ``active_registry`` into the shared state for downstream nodes.

classifier.py — ``classifier_node``
    Routes each user request to the correct processing layer using a **three-tier**
    strategy that escalates in cost only when needed:

    +---------+-------------------------------------------+------------------+
    | Tier    | Mechanism                                 | Cost             |
    +=========+===========================================+==================+
    | NLU     | Regex pattern matching (``INTENT_PATTERNS``) | Negligible    |
    | SLM     | Weighted keyword scoring (``SLM_WEIGHTS``)   | Very low      |
    | LLM     | Semantic fallback (always → local_supervisor)| Low (static) |
    +---------+-------------------------------------------+------------------+

    Recognised ``target_layer`` values: ``"local_supervisor"``, ``"remote_agent"``.

registry.py — ``get_available_agents()``
    Returns a dictionary of agent metadata records (name, description, tags,
    URL, capabilities, version, status).  Currently a mock; in production this
    would query a Document DB or a service-discovery endpoint.

    Registered agents: ``local_supervisor``, ``remote_agent``,
    ``spending_analysis``, ``anomaly_detection``, ``cash_flow_forecasting``.
"""
