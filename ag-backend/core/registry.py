def get_available_agents() -> dict:
    """
    Mock dynamic agent registry.
    Returns agent metadata, endpoints, and capabilities simulating a Document DB lookup.
    """
    return {
        "local_supervisor": {
            "name": "Local_Supervisor",
            "description": "Primary task router and parallel delegator for financial agents.",
            "tags": ["core", "routing", "finance"],
            "owners": ["Platform_Team"],
            "url": "local://supervisor",
            "authentication": "None",
            "status": "online",
            "capabilities": ["task_planning", "delegation", "aggregation"],
            "version": "1.2.0"
        },
        "remote_agent": {
            "name": "Remote_Compliance_Agent",
            "description": "Cross-network compliance checker.",
            "tags": ["compliance", "remote", "verification"],
            "owners": ["Security_Team"],
            "url": "https://api.internal.corp/v1/agent3",
            "authentication": "OAuth2",
            "status": "online",
            "capabilities": ["external_api_call", "compliance_check"],
            "version": "0.9.1"
        },
        "spending_analysis": {
            "name": "Spending_Analysis",
            "description": "Categorizes and sums transaction arrays.",
            "tags": ["finance", "analysis", "local"],
            "owners": ["Data_Science_Team"],
            "url": "local://spending_analysis",
            "authentication": "None",
            "status": "online",
            "capabilities": ["categorization", "summation"],
            "version": "2.0.1"
        },
        "anomaly_detection": {
            "name": "Anomaly_Detection",
            "description": "Scans expenditures for outliers or unused subscriptions.",
            "tags": ["finance", "security", "local"],
            "owners": ["Data_Science_Team"],
            "url": "local://anomaly_detection",
            "authentication": "None",
            "status": "online",
            "capabilities": ["outlier_detection", "unused_subscription_flagging"],
            "version": "1.1.5"
        },
        "cash_flow_forecasting": {
            "name": "Cash_Flow_Forecaster",
            "description": "Statistically projects future expenditure velocity.",
            "tags": ["finance", "forecasting", "local"],
            "owners": ["Data_Science_Team"],
            "url": "local://cash_flow_forecasting",
            "authentication": "None",
            "status": "online",
            "capabilities": ["statistical_projection"],
            "version": "1.0.0"
        }
    }
