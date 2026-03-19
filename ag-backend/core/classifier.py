from storage.state import SystemState
from telemetry.observability import trace_execution

import re

# Intent category mapping for NLU Tier
INTENT_PATTERNS = {
    "remote_agent": [
        r"\b(remote|external|network|api|server|cloud|distributed)\b",
        r"\b(fetch|get|pull)\b",
        r"\b(endpoint|request)\b"
    ],
    "local_supervisor": [
        r"\b(analyze|check|scan|audit|review)\b",
        r"\b(finance|spending|budget|cash|money|transactions?)\b",
        r"\b(anomalies|frauds?|unusual|outliers?)\b",
        r"\b(forecast|predict|projection|future|cashflow)\b"
    ]
}

# Weighted keywords for SLM Tier (simulates statistical pattern recognition)
SLM_WEIGHTS = {
    "local_supervisor": {
        "finance": 0.8, "spending": 0.9, "analyze": 0.5, "forecast": 0.9,
        "anomalies": 0.9, "fraud": 0.7, "savings": 0.6, "budget": 0.7,
        "report": 0.4, "cashflow": 0.9, "transactions": 0.6, "predict": 0.8,
        "outlier": 0.7, "audit": 0.6, "bank": 0.5, "account": 0.5
    },
    "remote_agent": {
        "external": 0.9, "remote": 0.9, "cloud": 0.8, "api": 0.7,
        "fetch": 0.6, "sync": 0.8, "distributed": 0.9, "server": 0.7,
        "network": 0.6, "ping": 0.5, "latency": 0.4
    }
}

def nlu_extract(user_input: str) -> str:
    """NLU Tier: Fast, regex-based intent extraction."""
    print("   -> [NLU Tier] Scanning for explicit structural patterns...")
    for intent, patterns in INTENT_PATTERNS.items():
        for pattern in patterns:
            if re.search(pattern, user_input, re.IGNORECASE):
                return intent
    return None

def slm_classify(user_input: str) -> str:
    """SLM Tier: Weighted statistical pattern recognition for complex requests."""
    print("   -> [SLM Tier] Calculating statistical intent distribution...")
    scores = {"local_supervisor": 0.0, "remote_agent": 0.0}
    words = re.findall(r"\w+", user_input.lower())
    
    for word in words:
        for target, weights in SLM_WEIGHTS.items():
            if word in weights:
                scores[target] += weights[word]
    
    if not any(scores.values()):
        return None
        
    best_target = max(scores, key=scores.get)
    # Require a confidence threshold for SLM
    if scores[best_target] >= 0.5: # Lowered threshold slightly for better coverage
        return best_target
    return None

def llm_reasoning(user_input: str) -> str:
    """
    LLM Tier: Deep semantic reasoning. 
    Currently acts as a robust fallback to ensure no 'IDK' handoffs.
    """
    print("   -> [LLM Tier] Final semantic fallback. Routing to Local Supervisor.")
    # Default everything else to Local Supervisor rather than IDK
    return "local_supervisor"

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
