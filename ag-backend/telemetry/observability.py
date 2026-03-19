import hashlib
import functools


def check_llm_safety_guardrails(prompt: str) -> None:
    """Mock LLM Safety Filter to block malicious prompt injections before routing."""
    malicious_patterns = ["ignore all previous instructions", "bypass system prompt", "DROP TABLE"]
    for pattern in malicious_patterns:
        if pattern in prompt.lower():
            print(f"[Security] Potential prompt injection detected: '{pattern}'. Blocking execution.")
            raise ValueError("Prompt blocked by safety classifiers.")
    print("[Security] System Safety Prompts validated. No malicious intent detected.")

def trace_execution(func):
    """Decorator to simulate OpenTelemetry/LangSmith tracing with Audit Logging and Correlation IDs."""
    @functools.wraps(func)
    def wrapper(*args, **kwargs):
        caller = kwargs.get("caller_identity", "unknown_agent")
        correlation_id = "unknown_session"
        
        # Extract Correlation_ID from the STM state if available
        if args and isinstance(args[0], dict) and "messages" in args[0]:
            messages = args[0]["messages"]
            if messages:
                last_msg = messages[-1]
                if isinstance(last_msg, dict) and "meta" in last_msg:
                    correlation_id = last_msg["meta"].get("session_id", correlation_id)
                elif hasattr(last_msg, "meta"):
                     correlation_id = last_msg.meta.get("session_id", correlation_id)
                     
        input_hash = hashlib.sha256(str(args).encode()).hexdigest()[:8]
        
        print(f"[Observability] [CorrID: {correlation_id}] Span started for {func.__name__} by {caller}.")
        result = func(*args, **kwargs)
        
        output_hash = hashlib.sha256(str(result).encode()).hexdigest()[:8]
        print(f"[Observability] [CorrID: {correlation_id}] Span closed. Output Hash: {output_hash}.")
        return result
    return wrapper
