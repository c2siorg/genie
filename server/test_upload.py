# GENERATED USING LLM TO TEST FILE ENDPOINT
import requests
import json
import uuid

# Configuration
API_URL = "http://localhost:8000/api/v1/transactions/upload"
CSV_FILE = "dummy_transactions.csv"
USER_ID = str(uuid.uuid4())  # Generate a random user ID for RLS testing

def test_upload():
    print(f"🚀 Starting test upload for user: {USER_ID}")
    
    # Headers include the user_id which the CBAC and RLS use
    headers = {
        "user-id": USER_ID
    }
    
    with open(CSV_FILE, "rb") as f:
        files = {"file": (CSV_FILE, f, "text/csv")}
        
        try:
            response = requests.post(API_URL, files=files, headers=headers)
            
            if response.status_code == 202 or response.status_code == 200:
                print("✅ Upload Successful!")
                result = response.json()
                print(f"   Status: {result.get('status')}")
                print(f"   Task ID: {result.get('task_id')}")
                print(f"   Rows Received: {result.get('rows_received')}")
                print(f"   PII Entities Secured: {result.get('pii_entities_secured')}")
                
            else:
                print(f"❌ Upload Failed with status code: {response.status_code}")
                print(response.text)
                
        except requests.exceptions.ConnectionError:
            print("❌ Failed to connect to the API. Is the docker container running?")

if __name__ == "__main__":
    test_upload()
    print("\n💡 Tip: Check the worker logs to see the PII sanitization in action:")
    print("   docker compose logs -f worker")
