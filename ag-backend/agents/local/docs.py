"""
agents.local — On-Device Specialist Agent Nodes
================================================

The ``agents.local`` package contains all agent nodes that execute locally
(i.e., on the same machine as the backend).  They are invoked by the
``local_supervisor_node`` in a sequential fan-out pattern.

Modules
-------

supervisor.py — ``local_supervisor_node``
    The entry-point for the local processing branch.  Responsibilities:

    1. Runs ``spending_analysis_node`` first (prerequisite step).
    2. Fan-outs to ``anomaly_detection_node`` and ``cash_flow_forecasting_node``.
    3. Merges all three outputs into a single state dictionary that is returned
       to the graph for downstream synthesis by ``reasoning_llm_node``.

spending_analysis.py — ``spending_analysis_node``
    Aggregates raw transaction records by ``category`` and computes per-category
    totals plus an overall spend total.

    * **Input:** ``state["transactions"]`` — list of transaction dicts.
    * **Output:** ``state["spending_summary"]`` — human-readable multi-line string.

anomaly_detection.py — ``anomaly_detection_node``
    Detects unusual transactions using two complementary strategies:

    * **Rule-based (stub):** Placeholder loop ready to be filled with custom
      threshold rules (e.g., large-amount flags, subscription over-spend).
    * **Isolation Forest (ML):** Uses ``sklearn.ensemble.IsolationForest`` with
      ``contamination=0.05`` to flag statistical outliers on the ``amount``
      feature.  Falls back gracefully when fewer than 10 transactions are
      available.

    * **Input:** ``state["transactions"]``
    * **Output:** ``state["anomalies_detected"]`` (rule-based list),
      ``state["isolation_forest_anomalies"]`` (ML list).

cash_flow_forecasting.py — ``cash_flow_forecasting_node``
    Projects future monthly expenses using two time-series models:

    * **ARIMA** (via ``pmdarima.auto_arima``): Adaptive model selection;
      uses seasonal ARIMA when ≥ 36 months of history are available.
    * **Prophet** (via ``prophet.Prophet``): Yearly-seasonality model with
      uncertainty intervals (``yhat_lower`` / ``yhat_upper``).

    Both models require at least **5 months** of aggregated history.  The node
    formats 6-month forecasts as human-readable strings.

    * **Input:** ``state["transactions"]``
    * **Output:** ``state["arima_forecast"]``, ``state["prophet_forecast"]``

recommendation.py — ``recommendation_node`` *(currently unused)*
    Rule-based recommendation synthesiser that inspects anomalies and the
    spending summary to produce a short list of actionable suggestions
    (e.g., cancel unused subscriptions, reduce food-delivery spend).
    Disabled in the current supervisor fan-out but available for future use.

reasoning_llm.py — ``reasoning_llm_node``
    Synthesises all agent outputs into a structured, human-readable financial
    report.

    * Attempts to call a real LLM via API key when one is set (extensible hook).
    * Falls back to ``_synthesize_with_template()``, which produces a
      box-drawing ASCII report with sections for executive summary, spending
      overview, anomaly alerts, forecast outlook, and key recommendations.

    * **Input:** full ``SystemState``
    * **Output:** ``state["reasoning_output"]`` — fully formatted report string.
"""
