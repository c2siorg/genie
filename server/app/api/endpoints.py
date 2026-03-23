import csv
import io
import uuid
from fastapi import APIRouter, UploadFile, File, Depends, HTTPException
from app.api.dependencies import verify_cbac, rate_limiter
from app.api.pii import sanitize_transaction_row
from app.worker.tasks import process_transaction_batch

router = APIRouter()

async def log_audit_events(user_id: str, logs: list, task_id: str):
    """Pushes audit logs to a secure compliance table."""
    # Implementation: Insert structured JSON into audit_logs table
    pass

@router.post("/upload")
async def upload_transactions(
    file: UploadFile = File(...),
    context: dict = Depends(verify_cbac),
    _ = Depends(rate_limiter)
):
    """
    Expects a CSV upload. Parses and processes records with ingestion stats.
    """
    filename = file.filename or ""
    if not filename.endswith(".csv"):
        raise HTTPException(status_code=400, detail="Invalid file type. Only .csv is allowed.")
        
    content = await file.read()
    string_data = content.decode("utf-8")
    
    try:
        reader = list(csv.DictReader(io.StringIO(string_data)))
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Failed to parse CSV: {str(e)}")
        
    batch = []
    audit_logs: list[dict] = []
    
    # 1. PII Detection & Tokenization
    for row in reader:
        clean_row = sanitize_transaction_row(row, context["user_id"], audit_logs)
        batch.append(clean_row)
        
    # 2. Generate a tracking ID
    task_id = str(uuid.uuid4())

    # 3. Process batch with duplicate detection and ingestion stats
    processing_stats = await process_transaction_batch(batch, str(context["user_id"]))
    
    # 4. Log PII modifications for compliance/forensics
    await log_audit_events(context["user_id"], audit_logs, task_id)
    
    return {
        "status": "completed" if not processing_stats.get("error") else "failed",
        "task_id": task_id,
        "rows_received": len(batch),
        "rows_processed": processing_stats.get("processed", 0),
        "rows_skipped": processing_stats.get("skipped", 0),
        "duplicate_rows": processing_stats.get("duplicate_count", 0),
        "duplicate_in_batch": processing_stats.get("duplicate_in_batch", 0),
        "duplicate_existing": processing_stats.get("duplicate_existing", 0),
        "missing_transaction_id": processing_stats.get("missing_transaction_id", 0),
        "pii_entities_secured": len(audit_logs),
        "message": (
            f"Processed batch with {processing_stats.get('processed', 0)} inserts, "
            f"{processing_stats.get('duplicate_count', 0)} duplicates skipped, "
            f"and {processing_stats.get('missing_transaction_id', 0)} rows missing transaction_id."
        ),
        "error": processing_stats.get("error"),
    }
