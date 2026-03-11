from storage.state import SystemState

def anomaly_detection_node(state: SystemState) -> dict:
    """Agentic node to detect anomalies like unused subscriptions or unusual spikes."""
    print("[Anomaly Detection Agent] Scanning for unusual patterns...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        return {"anomalies_detected": []}
    
    anomalies = []
    
    # Mock simple rule: if a single transaction is > $500, flag it.
    for t in transactions:
        desc = t.get("description", "").lower()
        amount = t.get("amount", 0.0)
        
        if amount > 500:
            anomalies.append(f"Unusually large transaction: {t['description']} for ${amount}")
        
        # Mock unused sub detection
        if "subscription" in desc or "netflix" in desc or "gym" in desc:
            # Let's pretend it finds something unused based on history
            anomalies.append(f"Potential unused subscription: {t['description']} (${amount}/month)")
            
    print(f"   -> Found {len(anomalies)} anomalies.")
    return {"anomalies_detected": anomalies}
