from storage.state import SystemState

def recommendation_node(state: SystemState) -> dict:
    """Agentic node that aggregates data and produces final recommendations using LLM / Rules."""
    print("[Recommendation Engine] Synthesizing insights into actionable steps...")
    
    anomalies = state.get("anomalies_detected", [])
    spending_summary = state.get("spending_summary", "")
    
    recommendations = []
    
    if list(filter(lambda x: "unused subscription" in x.lower(), anomalies)):
        recommendations.append("Review and cancel potentially unused subscriptions identified in anomalies.")
        
    if "Food" in spending_summary and "Delivery" in spending_summary:
        recommendations.append("Consider reducing food delivery to lower overall monthly expenses.")
        
    if not recommendations:
        recommendations.append("Spending looks well-balanced. Ensure you are meeting your savings targets (e.g., 20% rule).")
        
    print(f"   -> Synthesized {len(recommendations)} recommendations.")
    return {"recommendations": recommendations}
