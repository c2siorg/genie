import os
from storage.state import SystemState
from telemetry.observability import trace_execution


def _synthesize_with_template(state: dict) -> str:
    """Fallback template-based synthesis when no LLM API key is available."""

    spending = state.get("spending_summary", "No spending data.")
    rule_anomalies = state.get("anomalies_detected", [])
    if_anomalies = state.get("isolation_forest_anomalies", [])
    arima = state.get("arima_forecast", "No forecast.")
    prophet = state.get("prophet_forecast", "No forecast.")

    total_anomalies = len(rule_anomalies) + len(if_anomalies)

    # Build the report
    lines = []
    lines.append("╔" + "═" * 68 + "╗")
    lines.append("║" + "  🧠 GENIE AI — FINANCIAL INTELLIGENCE REPORT".ljust(68) + "║")
    lines.append("╚" + "═" * 68 + "╝")
    lines.append("")

    # 1. Executive Summary
    lines.append("┌─────────────────────────────────────────────────────────────────────┐")
    lines.append("│  📋 EXECUTIVE SUMMARY                                              │")
    lines.append("└─────────────────────────────────────────────────────────────────────┘")
    lines.append(f"  Analysis complete across spending, anomaly detection, and forecasting")
    lines.append(f"  agents. {total_anomalies} anomalies flagged ({len(rule_anomalies)} rule-based,")
    lines.append(f"  {len(if_anomalies)} ML-detected). Forecasts generated via ARIMA & Prophet.")
    lines.append("")

    # 2. Spending Overview
    lines.append("┌─────────────────────────────────────────────────────────────────────┐")
    lines.append("│  💰 SPENDING OVERVIEW                                              │")
    lines.append("└─────────────────────────────────────────────────────────────────────┘")
    for line in spending.split("\n"):
        lines.append(f"  {line}")
    lines.append("")

    # 3. Anomaly Alert
    lines.append("┌─────────────────────────────────────────────────────────────────────┐")
    lines.append("│ ANOMALY ALERT                                                  │")
    lines.append("└─────────────────────────────────────────────────────────────────────┘")

    if rule_anomalies:
        lines.append(f"  Rule-Based Detections ({len(rule_anomalies)}):")
        # Show top 10 unique
        seen = set()
        count = 0
        for a in rule_anomalies:
            key = a[:60]
            if key not in seen and count < 10:
                lines.append(f"    ⚠  {a}")
                seen.add(key)
                count += 1
        if len(rule_anomalies) > 10:
            lines.append(f"    ... and {len(rule_anomalies) - 10} more rule-based anomalies")
    else:
        lines.append("  Rule-Based: No anomalies detected ✅")

    lines.append("")

    if if_anomalies:
        lines.append(f"  Isolation Forest ML Detections ({len(if_anomalies)}):")
        for a in if_anomalies[:10]:
            lines.append(f"    🔍 {a}")
        if len(if_anomalies) > 10:
            lines.append(f"    ... and {len(if_anomalies) - 10} more ML-detected anomalies")
    else:
        lines.append("  Isolation Forest ML: No anomalies detected ✅")

    lines.append("")

    # 4. Forecast Outlook
    lines.append("┌─────────────────────────────────────────────────────────────────────┐")
    lines.append("│  📈 FORECAST OUTLOOK                                               │")
    lines.append("└─────────────────────────────────────────────────────────────────────┘")
    lines.append("  [ARIMA Model]")
    for line in arima.split("\n"):
        lines.append(f"    {line}")
    lines.append("")
    lines.append("  [Prophet Model]")
    for line in prophet.split("\n"):
        lines.append(f"    {line}")
    lines.append("")

    # 5. Key Recommendations
    lines.append("┌─────────────────────────────────────────────────────────────────────┐")
    lines.append("│  💡 KEY RECOMMENDATIONS                                            │")
    lines.append("└─────────────────────────────────────────────────────────────────────┘")

    recommendations = []
    if any("subscription" in a.lower() for a in rule_anomalies):
        recommendations.append("Review and cancel unused subscriptions to save on recurring expenses.")
    if any("large transaction" in a.lower() or "unusually large" in a.lower() for a in rule_anomalies):
        recommendations.append("Investigate large transactions for unauthorized or accidental charges.")
    if if_anomalies:
        recommendations.append("Review ML-flagged transactions — Isolation Forest detected statistical outliers that may need attention.")
    recommendations.append("Ensure monthly income covers projected expenses from both ARIMA and Prophet forecasts.")
    recommendations.append("Build an emergency fund covering at least 3 months of projected expenses.")

    for i, rec in enumerate(recommendations, 1):
        lines.append(f"  {i}. {rec}")

    lines.append("")
    lines.append("─" * 70)
    lines.append("  Report compiled by Genie AI Agent Workflow")
    lines.append("  Models used: Isolation Forest | ARIMA | Prophet")
    lines.append("─" * 70)

    return "\n".join(lines)


@trace_execution
def reasoning_llm_node(state: SystemState) -> dict:
    """
    Reasoning LLM node that compiles all agent outputs into a friendly,
    human-readable financial report. Templated at the moment.. 
    """
    print("[Reasoning LLM Agent] Compiling all agent data into final report...")

    report = None

    if not report:
        print("   -> Falling back to template-based synthesis...")
        report = _synthesize_with_template(state)

    print("   -> Final report compiled.")
    return {"reasoning_output": report}
