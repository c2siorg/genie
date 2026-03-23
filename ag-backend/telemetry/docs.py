"""
telemetry — Observability, Tracing & Safety Guardrails
=======================================================

The ``telemetry`` package provides cross-cutting concerns that apply to every
agent node in the system: **distributed tracing**, **audit logging**, and
**LLM safety / prompt-injection filtering**.

These utilities are designed to be transparent — agents apply them via a
decorator and do not need to manage tracing state manually.

Modules
-------

observability.py
    Provides two public utilities:

    ``@trace_execution`` *(decorator)*
        Wraps any LangGraph node function with span-level observability
        inspired by **OpenTelemetry** and **LangSmith**.

        On each call it:

        1. Extracts the ``session_id`` from the last ``STM_Message`` in state
           to use as a **correlation ID** (falls back to
           ``"unknown_session"`` when unavailable).
        2. Computes a **SHA-256 input hash** (first 8 hex chars) for
           tamper-evident audit logging of inputs.
        3. Logs a "Span started" message with the correlation ID, function
           name, and caller identity.
        4. Calls the wrapped function.
        5. Computes a **SHA-256 output hash** and logs a "Span closed" message.

        Usage::

            from telemetry.observability import trace_execution

            @trace_execution
            def my_agent_node(state: SystemState) -> dict:
                ...

    ``check_llm_safety_guardrails(prompt: str) -> None``
        A **prompt-injection safety filter** that inspects the user prompt for
        known malicious patterns before it is forwarded to any LLM or routing
        layer.

        Blocked patterns (case-insensitive):

        * ``"ignore all previous instructions"``
        * ``"bypass system prompt"``
        * ``"DROP TABLE"``

        Raises ``ValueError`` immediately if a pattern is matched, halting
        execution.  Logs a confirmation message when no threat is found.

Extension Points
----------------
* Replace the ``print``-based logging with a real OpenTelemetry
  ``tracer.start_as_current_span`` context manager.
* Export spans to a backend (Jaeger, Zipkin, LangSmith) via OTLP.
* Extend ``check_llm_safety_guardrails`` with a trained classifier or
  an external moderation API call for production-grade safety.
"""
