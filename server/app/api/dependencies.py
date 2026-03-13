import time
from fastapi import HTTPException, Header, Depends
from typing import Dict, Any
from collections import defaultdict, deque

rate_limit_data = defaultdict(deque)

async def verify_cbac(user_id: str = Header(default="00000000-0000-0000-0000-000000000000")) -> Dict[str, Any]:
    """
    Check a mock user context dictionary.
    Raises HTTP 403 if 'upload_transactions' is not in the user's permissions.
    """
    context = {
        "user_id": user_id,
        "permissions": ["upload_transactions", "read_transactions"]
    }
    
    if "upload_transactions" not in context["permissions"]:
        raise HTTPException(status_code=403, detail="Forbidden: missing upload_transactions permission")
    
    return context

async def rate_limiter(context: dict = Depends(verify_cbac)):
    """
    Tracks CSV uploads by user_id and blocks if > 5 uploads in 60s.
    Uses in-memory storage.
    """
    user_id = context["user_id"]
    window_seconds = 60
    max_requests = 5
    current_time = time.time()
    
    # Clean old requests for this user
    user_requests = rate_limit_data[user_id]
    while user_requests and user_requests[0] < current_time - window_seconds:
        user_requests.popleft()
        
    if len(user_requests) >= max_requests:
        raise HTTPException(status_code=429, detail="Rate limit exceeded. Maximum 5 uploads per 60-second window.")
        
    user_requests.append(current_time)
    
    return True
