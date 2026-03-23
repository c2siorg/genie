"""
integration — Model Context Protocol (MCP) Gateway
====================================================

The ``integration`` package implements the **Model Context Protocol (MCP)**
security and routing layer that governs how internal tools (database queries,
specialist computations, etc.) are invoked by agents in a controlled,
auditable way.

Why MCP?
--------
Rather than giving agents unrestricted access to internal functions, every
tool invocation is routed through the ``mcp_gateway``, which enforces
*authentication* and *role-based access control (RBAC)* before delegating
to the ``tool_adapter``.  This prevents privilege escalation, prompt-injection
tool misuse, and unintended data access.

Modules
-------

mcp_protocol.py
    Provides two public functions:

    ``mcp_gateway(tool_name, parameters, caller_identity, auth_token) -> str``
        The single entry-point for all MCP tool calls.  Processing steps:

        1. **Authentication** – validates that ``auth_token`` is a valid
           ``"Bearer <token>"`` string; returns ``"401 Unauthorized"`` otherwise.
        2. **RBAC check** – enforces access-control rules; for example, the
           ``local_supervisor`` is denied access to ``high_security_db``,
           returning ``"403 Forbidden"``.
        3. **Routing** – on success, delegates to ``tool_adapter`` and returns
           its result.

    ``tool_adapter(tool_name, parameters) -> str``
        Internal function that maps an MCP ``tool_name`` to the actual
        implementation.  Currently returns a mock result string; in production
        this would dispatch to the appropriate internal module or external
        service.

Extension Points
----------------
* Add new RBAC rules to the ``mcp_gateway`` to restrict or permit agents.
* Extend ``tool_adapter`` with a dispatch table mapping tool names to real
  Python callables or HTTP endpoints.
* Integrate with a real token-validation service (JWT, OAuth2 introspection)
  to replace the current string-prefix check.
"""
