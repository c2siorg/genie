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
    content: str
    meta: MessageMeta

# Agent State
class AgentRuntimeState(TypedDict):
    agent_states: Dict[str, Any]
    spending_summary: str
    anomalies_detected: List[str]
    cash_flow_prediction: str
    recommendations: List[str]

# Overall System State combining segregated stores
class SystemState(TypedDict):
    # Conversation History Domain
    messages: Annotated[Sequence[STM_Message], operator.add]
    current_intent: str
    target_layer: str
    
    # Agent State Domain
    agent_states: Dict[str, Any]
    spending_summary: str
    anomalies_detected: List[str]
    cash_flow_prediction: str
    recommendations: List[str]
    
    # Registry Domain
    active_registry: Dict[str, dict]
    
    # Core Data & Error handling
    transactions: List[Transaction]
    errors: List[str]
