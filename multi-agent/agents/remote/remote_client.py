from storage.state import SystemState
from langchain_core.messages import AIMessage
from telemetry.observability import trace_execution

@trace_execution
def remote_agent_node(state: SystemState) -> dict:
    print("[Remote Agent Layer] Firing API request to Agent #3 across network boundary...")
    return {"messages": [AIMessage(content="Result computed by Remote Agent #3")]}
