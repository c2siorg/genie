from langgraph.graph import StateGraph, START, END
from storage.state import SystemState
from core.orchestrator import orchestrator_node
from core.classifier import classifier_node
from agents.local.supervisor import local_supervisor_node
from agents.remote.remote_client import remote_agent_node
from telemetry.observability import trace_execution

@trace_execution
def build_graph():
    workflow = StateGraph(SystemState)
    
    # Add Architecture Nodes
    workflow.add_node("Orchestrator", orchestrator_node)
    workflow.add_node("Classifier", classifier_node)
    workflow.add_node("LocalSupervisor", local_supervisor_node)
    workflow.add_node("RemoteAgentClient", remote_agent_node)
    
    # Define the flow
    workflow.add_edge(START, "Orchestrator")
    workflow.add_edge("Orchestrator", "Classifier")
    
    # Conditional Edge based on Classifier
    def route_intent(state: SystemState):
        if state["target_layer"] == "local_supervisor":
            return "LocalSupervisor"
        elif state["target_layer"] == "remote_agent":
            return "RemoteAgentClient"
        return END

    workflow.add_conditional_edges("Classifier", route_intent)
    
    # Return to END based on the new simplistic architecture mockup
    workflow.add_edge("LocalSupervisor", END)
    workflow.add_edge("RemoteAgentClient", END)
    
    return workflow.compile()
