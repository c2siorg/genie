"""
knowledge — Vector Database & RAG Knowledge Layer
==================================================

The ``knowledge`` package provides the **Retrieval-Augmented Generation (RAG)**
infrastructure that grounds agent responses in verified financial rules and
dynamically selects the most relevant MCP tools for a given query.

In production this layer would connect to a **pgvector**-backed PostgreSQL
database (or an equivalent vector store such as Pinecone, Weaviate, or Chroma)
and perform approximate-nearest-neighbour (ANN) search over embedded documents.

Modules
-------

vector_db.py
    Provides three public functions:

    ``get_financial_rules(query: str) -> str``
        Performs a semantic search across a corpus of financial best-practice
        documents and returns the most relevant rule as a string.

        *Current implementation:* mock — always returns the 50/30/20
        budgeting rule regardless of the query, to exercise the interface
        without requiring a live vector DB.

        *Production implementation:* embed ``query`` with a sentence-transformer
        model, run ANN search on pgvector, and return the top-k documents.

    ``vector_search_tools(query: str) -> list[str]``
        Dynamically retrieves MCP tool names that are semantically relevant to
        the given query.  Agents should call this instead of hard-coding a
        fixed tool list so that the right tools are surfaced for each request.

        *Current implementation:* keyword heuristic —
        returns ``["internal_database_query"]`` for database-related queries
        and ``["generic_search"]`` otherwise.

    ``semantic_cache_check(query: str) -> str | None``
        Checks a semantic cache (e.g., Redis with vector similarity) to see
        whether a near-identical prompt has been answered recently, bypassing
        the LLM call entirely on cache hits.

        *Current implementation:* mock — returns a cached result only for the
        exact string ``"Budgeting best practices"``; returns ``None`` on misses.

Design Notes
------------
* These three functions together implement a **RAG + caching** pipeline that
  can significantly reduce LLM API costs and latency.
* The cache check should be the first step before any LLM call; tool search
  should happen before the agent decides which tools to use.
"""
