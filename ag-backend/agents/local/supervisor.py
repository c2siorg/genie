from storage.state import SystemState
from integration.mcp_protocol import mcp_gateway
from knowledge.vector_db import vector_search_tools
from agents.local.spending_analysis import spending_analysis_node
from agents.local.anomaly_detection import anomaly_detection_node
from agents.local.cash_flow_forecasting import cash_flow_forecasting_node
from agents.local.recommendation import recommendation_node
from telemetry.observability import trace_execution

@trace_execution
def local_supervisor_node(state: SystemState) -> dict:
    print("[Supervisor Agent] Decomposing task and delegating via secure channels...")
    
    # Context Engineering: Vector Search for tools before trying to use them
    messages = state.get("messages", [])
    query = ""
    if messages:
        last_msg = messages[-1]
        query = last_msg.get("content", "") if isinstance(last_msg, dict) else last_msg.content
        
    relevant_tools = vector_search_tools(query)
    
    # Execution of external MCP tool representation via Gateway
    if relevant_tools:
        tool_to_use = relevant_tools[0] # taking first mock hit
        tool_result = mcp_gateway(tool_to_use, {"id": 123}, "local_supervisor", "Bearer mock_token_123")
    else:
        tool_result = "No tools needed."
    
    print("-> [Message Bus] Initiating Parallel Fan-Out to Financial Agents...")
    # Delegation to the specialized financial agents (Parallel Fan-Out concept)
    updates = {}
    updates.update(spending_analysis_node(state))
    state.update(updates)
    
    updates.update(anomaly_detection_node(state))
    state.update(updates)
    
    updates.update(cash_flow_forecasting_node(state))
    state.update(updates)
    
    print("   -> [Message Bus] Fan-Out complete. Initiating Chained Sequence to Recommendation...")
    # Synthesis by Recommendation engine (Chained Sequence concept)
    updates.update(recommendation_node(state))
    state.update(updates)
    
    # Appending messages for history (STM format mapping deferred to main for simplicity)
    content = f"Supervisor aggregated result utilizing financial agents securely: {tool_result}"
    
    # In a real system, the payload would be matching STM_Message meta format
    from storage.state import STM_Message
    import uuid
    import datetime
    
    msg = STM_Message(
        content=content,
        meta={
            "session_id": str(uuid.uuid4()),
            "message_id": str(uuid.uuid4()),
            "user_id": "usr_999",
            "source": "local_supervisor",
            "timestamp": datetime.datetime.now().isoformat()
        }
    )
    
    updated_messages = list(state.get("messages", [])) + [msg]
    updates["messages"] = updated_messages
    return updates
