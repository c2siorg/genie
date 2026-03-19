import pandas as pd
from typing import List
from storage.state import SystemState
from telemetry.observability import trace_execution

def detect_with_isolation_forest(transactions: List[dict]) -> List[str]:
    """Detect anomalies using scikit-learn Isolation Forest."""
    if len(transactions) < 10:
        return ["Insufficient data for Isolation Forest (need 10+ transactions)."]

    from sklearn.ensemble import IsolationForest
    import numpy as np

    # Prepare features: spending amount only for this simple model
    df = pd.DataFrame(transactions)
    X = df[['amount']].values

    # Fit Isolation Forest
    # contamination=0.05 targets ~5% of data as outliers
    model = IsolationForest(contamination=0.05, random_state=42)
    preds = model.fit_predict(X) 
    
    # -1 is outlier, 1 is inlier
    outliers = df[preds == -1]
    
    anomalies = []
    for _, row in outliers.iterrows():
        # Get decision score (lower is more anomalous)
        score = model.decision_function([[row['amount']]])[0]
        anomalies.append(
            f"Anomalous transaction detected: {row.get('description', 'Unknown')} | "
            f"${row['amount']:.2f} | Category: {row.get('category', 'N/A')} | "
            f"Date: {row.get('date', 'N/A')} | Anomaly Score: {score:.4f}"
        )
    
    return anomalies

@trace_execution
def anomaly_detection_node(state: SystemState) -> dict:
    """Detect anomalies based on rule-based flags and Isolation Forest ML."""
    print("[Anomaly Detection Agent] Evaluating transactions...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        print("   -> No transactions provided for anomaly detection.")
        return {
            "anomalies_detected": [],
            "isolation_forest_anomalies": ["No transactions provided."]
        }

    anomalies = []

    # Rule-Based Detection
    for tx in transactions:
        amount = tx.get("amount", 0)
        desc = tx.get("description", "Unknown")
        cat = tx.get("category", "N/A")

        # Large Transaction Rule
        if amount > 1000:
            anomalies.append(f"[RuleBased] Unusually large transaction: {desc} for ${amount:,.2f}")
        
        # Subscription Multi-Large Rule
        if cat == "Subscription" and amount > 20:
             anomalies.append(f"[RuleBased] Potential unused subscription: {desc} (${amount:,.2f}/month)")

    # ML-Based Detection (Isolation Forest)
    print("   -> Running Isolation Forest ML model...")
    try:
        if_anomalies = detect_with_isolation_forest(transactions)
    except Exception as e:
        print(f"      Error in Isolation Forest: {e}")
        if_anomalies = [f"ML Model error: {str(e)}"]

    print(f"   -> Anomaly detection complete. Found {len(anomalies)} rule-based and {len(if_anomalies)} ML-based anomalies.")

    return {
        "anomalies_detected": anomalies,
        "isolation_forest_anomalies": if_anomalies
    }
