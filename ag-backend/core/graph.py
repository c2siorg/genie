from langgraph.graph import StateGraph, START, END
from storage.state import SystemState
from core.orchestrator import orchestrator_node
from core.classifier import classifier_node
from agents.local.supervisor import local_supervisor_node
from agents.local.reasoning_llm import reasoning_llm_node
from agents.remote.remote_client import remote_agent_node
from telemetry.observability import trace_execution

def build_graph():
    """Build the LangGraph workflow graph."""
    
    # 1. Initialize StateGraph with SystemState schema
    workflow = StateGraph(SystemState)

    # 2. Add Nodes
    workflow.add_node("orchestrator", orchestrator_node)
    workflow.add_node("classifier", classifier_node)
    
    # Federated Agent Nodes
    workflow.add_node("local_supervisor", local_supervisor_node)
    workflow.add_node("remote_agent", remote_agent_node)
    
    # Synthesis Node
    workflow.add_node("reasoning_llm", reasoning_llm_node)

    # 3. Define the DAG Structure (Edges)
    workflow.add_edge(START, "orchestrator")
    workflow.add_edge("orchestrator", "classifier")

    # 4. Routing logic from Classifier
    def route_to_target_layer(state: SystemState):
        target = state.get("target_layer", "local_supervisor")
        if target in ["local_supervisor", "remote_agent"]:
            return target
        return "local_supervisor"

    workflow.add_conditional_edges(
        "classifier",
        route_to_target_layer,
        {
            "local_supervisor": "local_supervisor",
            "remote_agent": "remote_agent",
            "END": END
        }
    )

    # 5. Connect Experts to synthesis and then END
    workflow.add_edge("local_supervisor", "reasoning_llm")
    workflow.add_edge("reasoning_llm", END)
    
    workflow.add_edge("remote_agent", END)

    # 6. Compile
    return workflow.compile()
