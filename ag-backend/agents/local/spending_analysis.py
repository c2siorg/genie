from storage.state import SystemState

def spending_analysis_node(state: SystemState) -> dict:
    """Agentic node to analyze spending patterns."""
    print("[Spending Analysis Agent] Analyzing transaction categories...")
    
    transactions = state.get("transactions", [])
    if not transactions:
        return {"spending_summary": "No transactions provided."}
    
    # Simple rule-based/statistical logic or LLM call could go here
    # Mock analysis
    category_totals = {}
    for t in transactions:
        cat = t.get("category", "Unknown")
        amount = t.get("amount", 0.0)
        category_totals[cat] = category_totals.get(cat, 0.0) + amount
    
    summary_lines = []
    total_spent = 0.0
    for cat, amount in sorted(category_totals.items(), key=lambda x: x[1], reverse=True):
        summary_lines.append(f" - {cat}: ${amount:.2f}")
        total_spent += amount
        
    summary = f"Total spend: ${total_spent:.2f}\n" + "\n".join(summary_lines)
    print(f"   -> Analyzed {len(transactions)} transactions.")
    
    return {"spending_summary": summary}
