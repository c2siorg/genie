from storage.state import SystemState
from telemetry.observability import trace_execution

def nlu_extract(user_input: str) -> str:
    """Mock NLU: Fast, regex or basic intent extraction."""
    print("   -> [NLU Tier] Attempting intent extraction...")
    if "external" in user_input or "remote" in user_input:
        return "remote_agent"
    return None

def slm_classify(user_input: str) -> str:
    """Mock SLM: Statistical pattern recognition for somewhat complex requests."""
    print("   -> [SLM Tier] Attempting pattern classification...")
    if "finance" in user_input or "spending" in user_input or "analyze" in user_input or "forecast" in user_input:
        return "local_supervisor"
    return None

def llm_reasoning(user_input: str) -> str:
    """Mock LLM: Deep reasoning for ambiguous queries."""
    print("   -> [LLM Tier] Attempting deep semantic routing...")
    if "help" in user_input:
        return "local_supervisor" # Let supervisor figure it out
    return "IDK"

@trace_execution
def classifier_node(state: SystemState) -> dict:
    """Classifies user intent using a tiered routing strategy to determine next layer."""
    messages = state.get("messages", [])
    if messages:
        # Compatibility check: if it's our new dict STM format or old object format
        last_msg = messages[-1]
        if isinstance(last_msg, dict):
            user_input = last_msg.get("content", "").lower()
        else:
            user_input = last_msg.content.lower()
    else:
        user_input = "analyze"
        
    print("[Classifier] Analyzing intent...")
    
    # Tiered approach: Use least expensive first
    target = nlu_extract(user_input)
    
    if not target:
        target = slm_classify(user_input)
        
    if not target:
        target = llm_reasoning(user_input)
        
    print(f"   -> Intent resolved with certainty. Routing to: {target}")
    return {"target_layer": target}
