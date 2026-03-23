"""
storage — Shared LangGraph State Schema
========================================

The ``storage`` package defines the **canonical data structures** that flow
through the LangGraph ``StateGraph``.  All agent nodes read from and write to
a single ``SystemState`` dictionary, ensuring type-safe, schema-validated
data exchange without requiring explicit message-passing plumbing between nodes.

Modules
-------

state.py
    Defines three ``TypedDict`` classes:

    ``Transaction``
        Represents a single financial transaction record.

        +--------------+--------+----------------------------------------------+
        | Field        | Type   | Description                                  |
        +==============+========+==============================================+
        | ``date``     | ``str``| ISO-format date string (e.g. ``"2024-03-15"``) |
        | ``description`` | ``str`` | Human-readable merchant / memo text       |
        | ``amount``   | ``float`` | Positive = expense, negative = income     |
        | ``category`` | ``str``| Spending category (e.g. ``"Food"``, ``"Subscription"``) |
        +--------------+--------+----------------------------------------------+

    ``STM_Message``
        Short-Term Memory message record stored in the conversation history.

        +---------------+--------+-------------------------------------------+
        | Field         | Type   | Description                               |
        +===============+========+===========================================+
        | ``id``        | ``str``| UUID of the message                       |
        | ``content``   | ``str``| Message text                              |
        | ``role``      | ``str``| ``"user"`` or ``"assistant"``             |
        | ``timestamp`` | ``str``| ISO-format timestamp                      |
        +---------------+--------+-------------------------------------------+

    ``SystemState``
        The **master graph state** ``TypedDict`` shared across all nodes.
        LangGraph accumulates ``messages`` via ``operator.add``; all other
        fields are overwritten on update.

        Key field groups:

        * **Conversation history:** ``messages``, ``current_intent``,
          ``target_layer``, ``user_id``, ``session_id``
        * **Agent outputs:** ``spending_summary``, ``anomalies_detected``,
          ``isolation_forest_anomalies``, ``arima_forecast``,
          ``prophet_forecast``, ``recommendations``, ``reasoning_output``
        * **Registry:** ``active_registry`` — populated by the orchestrator node
        * **Core data:** ``transactions`` — the raw input loaded from CSV
        * **Error handling:** ``errors`` — list of error strings for recovery
"""
