from storage.state import SystemState

def cash_flow_forecasting_node(state: SystemState) -> dict:
    """Agentic node to forecast future cash flow based on existing trends."""
    print("[Cash Flow Agent] Forecasting future cash flow...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        return {"cash_flow_prediction": "Insufficient data to predict cash flow."}
        
    total_spent = sum(t.get("amount", 0) for t in transactions)
    # Extremely basic mock prediction for next month
    projected = total_spent * 1.05  # Assume 5% increase based on mock inflation/trend
    
    prediction = f"Based on current velocity, expecting next month's expenses around ${projected:.2f}. "
    prediction += "Ensure your income covers this projected amount to maintain a positive cash flow."
    
    print(" -> Cash flow forecast generated.")
    return {"cash_flow_prediction": prediction}
