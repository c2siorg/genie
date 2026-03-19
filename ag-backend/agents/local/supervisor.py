# No unused import from core.registry
from agents.local.spending_analysis import spending_analysis_node
from agents.local.anomaly_detection import anomaly_detection_node
from agents.local.cash_flow_forecasting import cash_flow_forecasting_node
from storage.state import SystemState
from telemetry.observability import trace_execution

@trace_execution
def local_supervisor_node(state: SystemState) -> dict:
    """
    Local Supervisor node that orchestrates a fan-out to 2 experts.
    (Anomaly Detection and Cash Flow Forecasting).
    """
    print("[Local Supervisor] Breaking down complex financial analysis task...")
    
    # 1. First run the spending analysis since it's a prerequisite
    spending_data = spending_analysis_node(state)
    state.update(spending_data)
    
    # 2. Fan-out to 2 experts only (Recommendation disabled)
    print("   -> Initiating parallel fan-out to 2 agent models...")
    
    # Run Anomaly Detection
    anomaly_data = anomaly_detection_node(state)
    
    # Run Cash Flow Forecasting
    forecast_data = cash_flow_forecasting_node(state)
    
    # Merging outputs into state
    print("   -> Synthesis of expert agent outputs complete.")
    return {
        **spending_data,
        **anomaly_data,
        **forecast_data
    }
