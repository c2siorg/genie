import dateutil.parser
import logging
from sqlalchemy import text
from app.db.session import SessionLocal
from app.models.schema import Transaction, SemanticMemory
from app.worker.stubs import generate_embedding

logger = logging.getLogger(__name__)

async def process_transaction_batch(batch: list[dict], user_id: str):
    db = SessionLocal()
    try:
        # Enforce application context for RLS
        db.execute(text(f"SET app.current_user_id = '{user_id}'"))
        
        transactions_to_save = []
        memories_to_save = []
        
        for row in batch:
            txn_id = row.get("transaction_id")
            amount = float(row.get("amount", 0.0))
            merchant = row.get("merchant", "")
            description = row.get("merchant_description", "")
            date_str = row.get("date")
            try:
                txn_date = dateutil.parser.parse(date_str) if date_str else None
            except:
                txn_date = None
            
            # Create transaction object
            txn = Transaction(
                user_id=user_id,
                transaction_id=txn_id,
                amount=amount,
                merchant=merchant,
                date=txn_date
            )
            transactions_to_save.append(txn)
            
            # Use the sanitized description for semantic embeddings
            content = f"Spent ${amount} at {merchant}. Description: {description} on {txn_date}"
            embedding = generate_embedding(content)
            
            # Create semantic memory object
            mem = SemanticMemory(
                user_id=user_id,
                transaction_id=txn_id,
                content=content,
                embedding=embedding
            )
            memories_to_save.append(mem)
            
        # Bulk save
        db.bulk_save_objects(transactions_to_save)
        db.bulk_save_objects(memories_to_save)
        db.commit()
        logger.info(f"Successfully processed batch for user {user_id}")
        
    except Exception as exc:
        db.rollback()
        logger.error(f"Error processing batch for user {user_id}: {exc}")
        # In a real app, you might want to retry here or move to a dead-letter queue
    finally:
        db.close()
