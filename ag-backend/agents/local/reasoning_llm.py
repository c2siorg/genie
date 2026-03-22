import os
import requests
import json
from dotenv import load_dotenv
load_dotenv()
from storage.state import SystemState
from telemetry.observability import trace_execution

def build_prompt(spending_summary, anomalies_detected, isolation_forest_anomalies, arima_forecast, prophet_forecast):
    if not spending_summary:
        spending_summary = "No data available"
    if not anomalies_detected:
        anomalies_detected = "No rule based anomalies detected"
    else:
        anomalies_detected = "\n-".join(anomalies_detected)
    if not isolation_forest_anomalies:
        isolation_forest_anomalies = "No statistical anomalies detected"
    else:
        isolation_forest_anomalies = "\n-".join(isolation_forest_anomalies)
    if not arima_forecast:
        arima_forecast = "ARIMA forecast unavailable"
    if not prophet_forecast:
        prophet_forecast = "Prophet forecast unavailable"

    instruction = (f"You are a financial advisor generating a personal finance report for an everyday user."
              f"Be clear, concise, and actionable. A non-expert should understand every sentence."
              f"Generate a report with exactly these four sections:"
              f" 1. Spending Overview — based on the spending summary provided"
              f" 2. Anomaly Alert — based on rule-based and statistical anomalies provided"
              f" 3. Forecast Outlook — based on ARIMA and Prophet forecast provided"
              f" 4. Key Recommendations — based on all sections above"
              f" Format: each section has a 2-3 sentence summary followed by bullet"
              f" points for specific items where relevant. Critical constraint:never invent numbers, transactions, or facts."
              f" Use only what is explicitly provided in the data below.\n "
    )
    data = (f"--- USER FINANCIAL DATA --- \n"
              f"Spending Summary: \n{spending_summary}\n"
              f"Anomalies Detected: \n-{anomalies_detected}\n"
              f"Isolation Forest Anomalies: \n-{isolation_forest_anomalies}\n"
              f"Arima Forecast: \n{arima_forecast}\n"
              f"Prophet Forecast: \n{prophet_forecast}"
            )
    return instruction, data

def post_data(instruct, d, url):
    r = requests.post(
        url,
        headers={
        "Authorization": f"Bearer {os.getenv('AUTH_TOKEN')}",
        "Content-Type": "application/json"
    },
        data=json.dumps(
            {
                "model": "nvidia/nemotron-3-super-120b-a12b:free",
                "messages": [
                    {
                        "role": "system",
                        "content": f"{instruct}"
                    },
                    {
                        "role": "user",
                        "content": f"{d}"
                    }
                ]
            })
    )
    r.raise_for_status()
    r = r.json()['choices'][0]['message']['content']
    return r

def call_llm(instruction, data):
    try:
        response = post_data(instruction, data, "https://openrouter.ai/api/v1/chat/completions")
        return response

    except requests.exceptions.Timeout:
        print("Timeout — try again")

    except requests.exceptions.RequestException as e:
        print(f"LLM Call Failed: {e}")
        return None

def synthesise(state:SystemState) -> str:

    spending_summary = state.get("spending_summary")
    anomalies_detected = state.get("anomalies_detected")
    isolation_forest_anomalies = state.get("isolation_forest_anomalies")
    arima_forecast = state.get("arima_forecast")
    prophet_forecast = state.get("prophet_forecast")

    instruction, data = build_prompt(spending_summary, anomalies_detected, isolation_forest_anomalies, arima_forecast, prophet_forecast)

    report = call_llm(instruction, data)
    print(f"[synthesise] LLM returned: {'SUCCESS' if report else 'None - will fallback'}")
    return report


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

    report = synthesise(state)

    if not report:
        print("   -> Falling back to template-based synthesis...")
        report = _synthesize_with_template(state)

    print("   -> Final report compiled.")
    return {"reasoning_output": report}
