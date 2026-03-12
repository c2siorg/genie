import sys
import uuid
import datetime
import pandas as pd
from storage.state import STM_Message
from core.graph import build_graph
from knowledge.vector_db import get_financial_rules, semantic_cache_check
from telemetry.observability import redact_pii, check_llm_safety_guardrails

def load_transactions(csv_path: str) -> list:
    try:
        df = pd.read_csv(csv_path)
        return df.to_dict(orient="records")
    except Exception as e:
        print(f"Error loading CSV: {e}")
        return []

def main():
    print("=== Financial Transaction Analyst ===")
    
    if len(sys.argv) < 2:
        print("Usage: python main.py <path_to_csv>")
        sys.exit(1)
        
    csv_path = sys.argv[1]
    
    # 1. Load & Isolate Data
    raw_transactions = load_transactions(csv_path)
    if not raw_transactions:
        return
    
    transactions = []
    for t in raw_transactions:
        transactions.append(t)
        
    print(f"Loaded and sanitized {len(transactions)} transactions.")
    
    # 2. Context Engineering (Semantic Cache & RAG)
    user_query = "Please analyze my transactions and give Budgeting best practices"
    
    # Check strict LLM Safety Guardrails before processing the query
    check_llm_safety_guardrails(user_query)
    
    cached_response = semantic_cache_check(user_query)
    if cached_response:
        rules = cached_response
    else:
        rules = get_financial_rules(user_query)
        
    # Create STM payload
    msg = STM_Message(
        content=f"{user_query}. Context: {rules}",
        meta={
            "session_id": str(uuid.uuid4()),
            "message_id": str(uuid.uuid4()),
            "user_id": "usr_999",
            "source": "user_application",
            "timestamp": datetime.datetime.now().isoformat()
        }
    )
    
    # 3. Initialize State
    initial_state = {
        "messages": [msg],
        "transactions": transactions,
        "agent_states": {},
        "active_registry": {},
        "spending_summary": "",
        "anomalies_detected": [],
        "cash_flow_prediction": "",
        "recommendations": [],
        "current_intent": "analyze",
        "target_layer": "",
        "errors": []
    }

    
    # 4. Build and Run Graph
    app = build_graph()
    
    # Execute the workflow
    print("\n--- Starting Data Processing Workflow ---")
    
    # Get final state from the graph using invoke format
    final_state = app.invoke(initial_state)
    
    print("\n--- Workflow Complete ---")
    
    # 5. Output Insights
    print("\nFinal Genie Insights:")
    print("=" * 40)
    print("\nSpending Summary:")
    print(final_state.get("spending_summary", ""))
    
    print("\nAnomalies Identified:")
    for a in final_state.get("anomalies_detected", []):
        print(f" - {a}")
        
    print("\nCash Flow Forecast:")
    print(final_state.get("cash_flow_prediction", ""))
    
    print("\nRecommendations:")
    for r in final_state.get("recommendations", []):
        print(f" - {r}")
        
    print("\n========================================")

if __name__ == "__main__":
    main()
