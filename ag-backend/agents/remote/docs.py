"""
agents.remote — Remote / Federated Agent Client
================================================

The ``agents.remote`` package provides the client-side adapter for invoking
external agents that live outside the local process boundary — for example,
cloud-hosted compliance engines, cross-network verification services, or
third-party financial data APIs.

Modules
-------

remote_client.py — ``remote_agent_node``
    A LangGraph node stub that represents communication with a remote agent.

    Current behaviour
        Prints a trace message and returns a mock ``AIMessage`` with
        ``"Result computed by Remote Agent #3"`` as its content.  This keeps
        the graph topology valid while the real HTTP/gRPC transport layer is
        implemented.

    Intended behaviour (production)
        * Authenticate using OAuth2 (see ``core.registry`` for the endpoint
          and auth metadata).
        * Serialise the relevant state fields into a JSON payload.
        * POST to ``https://api.internal.corp/v1/agent3``.
        * Deserialise the response and merge it into the state.

    Routing
        The ``classifier_node`` in ``core.classifier`` routes requests to
        ``remote_agent_node`` when explicit remote/external/API keywords are
        detected in the user's message.  Unlike the local branch, the remote
        agent connects **directly** to ``END`` in the graph — its output is
        not passed through ``reasoning_llm_node``.
"""
