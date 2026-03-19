import operator
from typing import Annotated, Sequence, TypedDict, Dict, Any, List

class Transaction(TypedDict):
    date: str
    description: str
    amount: float
    category: str

# Conversation History Store concept (Short-Term Memory / STM)
class MessageMeta(TypedDict):
    session_id: str
    message_id: str
    user_id: str
    source: str # e.g. "orchestrator", "spending_agent", "user"
    timestamp: str

class STM_Message(TypedDict):
    id: str
    content: str
    role: str
    timestamp: str

# Agent State Domain Fields
class SystemState(TypedDict):
    # Conversation History Domain
    messages: Annotated[Sequence[STM_Message], operator.add]
    current_intent: str
    target_layer: str
    user_id: str
    session_id: str
    
    # Agent State Domain
    agent_states: Dict[str, Any]
    spending_summary: str
    anomalies_detected: List[str]
    isolation_forest_anomalies: List[str]
    arima_forecast: str
    prophet_forecast: str
    recommendations: List[str]
    reasoning_output: str
    
    # Registry Domain
    active_registry: Dict[str, dict]
    
    # Core Data & Error handling
    transactions: List[Transaction]
    errors: List[str]
