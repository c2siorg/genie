import dateutil.parser
import logging
from typing import Any
from sqlalchemy import text
from app.db.session import SessionLocal
from app.models.schema import Transaction, SemanticMemory
from app.worker.stubs import generate_embedding

logger = logging.getLogger(__name__)

async def process_transaction_batch(batch: list[dict], user_id: str):
    stats: dict[str, Any] = {
        "received": len(batch),
        "processed": 0,
        "skipped": 0,
        "duplicate_count": 0,
        "duplicate_in_batch": 0,
        "duplicate_existing": 0,
        "missing_transaction_id": 0,
    }

    db = SessionLocal()
    try:
        # Enforce application context for RLS
        db.execute(text(f"SET app.current_user_id = '{user_id}'"))

        unique_rows = []
        seen_transaction_ids = set()
        for row in batch:
            txn_id = row.get("transaction_id")
            if not txn_id:
                stats["missing_transaction_id"] += 1
                continue

            if txn_id in seen_transaction_ids:
                stats["duplicate_in_batch"] += 1
                continue

            seen_transaction_ids.add(txn_id)
            unique_rows.append(row)

        existing_ids = set()
        if unique_rows:
            candidate_ids = [row["transaction_id"] for row in unique_rows]
            existing_rows = (
                db.query(Transaction.transaction_id)
                .filter(Transaction.transaction_id.in_(candidate_ids))
                .all()
            )
            existing_ids = {row[0] for row in existing_rows}

        rows_to_insert = [
            row for row in unique_rows if row["transaction_id"] not in existing_ids
        ]
        stats["duplicate_existing"] = len(existing_ids)
        stats["duplicate_count"] = stats["duplicate_in_batch"] + stats["duplicate_existing"]
        stats["skipped"] = stats["duplicate_count"] + stats["missing_transaction_id"]
        
        transactions_to_save = []
        memories_to_save = []

        for row in rows_to_insert:
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
        if transactions_to_save:
            db.bulk_save_objects(transactions_to_save)
        if memories_to_save:
            db.bulk_save_objects(memories_to_save)
        db.commit()

        stats["processed"] = len(transactions_to_save)
        logger.info(
            "Batch processed for user %s | received=%d processed=%d skipped=%d duplicates=%d "
            "(batch=%d, existing=%d, missing_id=%d)",
            user_id,
            stats["received"],
            stats["processed"],
            stats["skipped"],
            stats["duplicate_count"],
            stats["duplicate_in_batch"],
            stats["duplicate_existing"],
            stats["missing_transaction_id"],
        )
        return stats

    except Exception as exc:
        db.rollback()
        logger.error(f"Error processing batch for user {user_id}: {exc}")
        # In a real app, you might want to retry here or move to a dead-letter queue
        return {**stats, "error": str(exc)}
    finally:
        db.close()
