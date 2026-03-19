import sys
import uuid
import datetime
import pandas as pd
from storage.state import STM_Message
from core.graph import build_graph

def load_transactions(csv_path: str) -> list:
    try:
        df = pd.read_csv(csv_path)
        return df.to_dict(orient="records")
    except Exception as e:
        print(f"Error loading CSV: {e}")
        return []

def main():
    print("\n" + "═" * 70)
    print(" 🧠 GENIE AI — SMART FINANCIAL TRANSACTION ANALYST")
    print("   Powered by Multi-Agent LangGraph Framework")
    print("═" * 70 + "\n")

    # 1. Load Data
    csv_path = sys.argv[1] if len(sys.argv) > 1 else "data/transactions.csv"
    print(f"📡 Loading transactions from: {csv_path}...")
    transactions = load_transactions(csv_path)
    
    if not transactions:
        print("❌ No transactions loaded. Exiting.")
        return

    print(f"✅ Loaded {len(transactions)} transactions.")

    # 2. Build the Initial State
    session_id = str(uuid.uuid4())
    user_id = "user_123"
    
    # Initialize the system state
    initial_state = {
        "user_id": user_id,
        "session_id": session_id,
        "transactions": transactions,
        "spending_summary": "",
        "anomalies_detected": [],
        "isolation_forest_anomalies": [],
        "arima_forecast": "",
        "prophet_forecast": "",
        "recommendations": [],
        "reasoning_output": "",
        "messages": [
            STM_Message(
                id=str(uuid.uuid4()),
                role="user",
                content="Analyze my recent transactions for anomalies and forecast my cash flow.",
                timestamp=datetime.datetime.now().isoformat()
            )
        ],
        "target_layer": "local_supervisor", # Start by routing to the local supervisor
        "errors": []
    }

    # 3. Compile and Run the Graph
    print("⚙️  Compiling financial analysis graph...")
    app = build_graph()

    # Execute the workflow
    print("\n" + "─" * 70)
    print("🚀 Starting AI Agent LangGraph Workflow")
    print("   Nodes: Orchestrator → Classifier → Supervisor → ReasoningLLM")
    print("   Agent Models: Anomaly Detection (IF + Rules) | Forecasting (ARIMA + Prophet)")
    print("─" * 70 + "\n")

    # Get final state from the graph
    final_state = app.invoke(initial_state)

    print("\n" + "─" * 70)
    print("✅ Workflow Complete")
    print("─" * 70)

    # 5. Output the AI-Compiled Report
    reasoning_output = final_state.get("reasoning_output", "")
    if reasoning_output:
        print("\n" + reasoning_output)
    else:
        # Fallback: show raw agent outputs
        print("\n⚠️  No reasoning output generated. Showing raw agent data:\n")

        print("💰 Spending Summary:")
        print(final_state.get("spending_summary", ""))

        print("\n🚨 Anomalies Detected:")
        for anomaly in final_state.get("anomalies_detected", []):
            print(f"  - {anomaly}")
        
        for ml_anomaly in final_state.get("isolation_forest_anomalies", []):
            print(f"  - [ML] {ml_anomaly}")

        print("\n📈 Cash Flow Forecast (ARIMA):")
        print(final_state.get("arima_forecast", "No ARIMA data."))

        print("\n📊 Cash Flow Forecast (Prophet):")
        print(final_state.get("prophet_forecast", "No Prophet data."))

    print("\n" + "═" * 70)
    print("  Report compiled at: " + datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"))
    print("═" * 70 + "\n")

if __name__ == "__main__":
    main()
