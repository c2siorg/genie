from storage.state import SystemState
from core.registry import get_available_agents
from telemetry.observability import trace_execution

@trace_execution
def orchestrator_node(state: SystemState) -> dict:
    """Central planner that decides how transactions are analyzed and manages lifecycle."""
    print("[Orchestrator] Updating Registry and maintaining context...")
    
    active_agents = get_available_agents()
    
    # 1. Error Recovery Mechanism
    errors = state.get("errors", [])
    if errors:
        print(f"[Orchestrator WARNING] Recovering from {len(errors)} previous errors: {errors[-1]}")
        # In a real system, we'd trigger a fallback path or self-correction mechanism
    
    # 2. Lifecycle Check - 'IDK' Fallback
    target_layer = state.get("target_layer", "")
    if target_layer == "IDK":
        print("[Orchestrator STOP] Classifier yielded 'IDK'. Triggering human handoff or asking for clarification.")
        # If IDK, we clear current intent to ask user
        return {"current_intent": "clarification_needed", "active_registry": active_agents}
        
    return {"active_registry": active_agents}
