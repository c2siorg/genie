import dateutil.parser
import logging
import uuid
from typing import Any
from sqlalchemy import text
from sqlalchemy.dialects.postgresql import insert
from app.db.session import SessionLocal
from app.models.schema import Transaction, SemanticMemory
from app.worker.stubs import generate_embedding

logger = logging.getLogger(__name__)

def process_transaction_batch(batch: list[dict], user_id: str) -> dict[str, Any]:
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
        try:
            user_uuid = uuid.UUID(str(user_id))
        except (ValueError, TypeError) as exc:
            return {**stats, "error": f"Invalid user_id UUID: {exc}"}

        # Enforce application context for RLS
        db.execute(
            text("SELECT set_config('app.current_user_id', :user_id, true)"),
            {"user_id": str(user_uuid)},
        )

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

        for row in unique_rows:
            txn_id = row.get("transaction_id")
            amount = float(row.get("amount", 0.0))
            merchant = row.get("merchant", "")
            description = row.get("merchant_description", "")
            date_str = row.get("date")
            try:
                txn_date = dateutil.parser.parse(date_str) if date_str else None
            except (ValueError, TypeError, OverflowError):
                txn_date = None

            # Race-safe idempotency: DB enforces uniqueness and ignores conflicts.
            tx_insert_result = db.execute(
                insert(Transaction)
                .values(
                    user_id=user_uuid,
                    transaction_id=txn_id,
                    amount=amount,
                    merchant=merchant,
                    date=txn_date,
                )
                .on_conflict_do_nothing(index_elements=["transaction_id"])
            )

            if tx_insert_result.rowcount == 0:
                stats["duplicate_existing"] += 1
                continue

            # Use the sanitized description for semantic embeddings.
            content = f"Spent ${amount} at {merchant}. Description: {description} on {txn_date}"
            embedding = generate_embedding(content)

            mem = SemanticMemory(
                user_id=user_uuid,
                transaction_id=txn_id,
                content=content,
                embedding=embedding,
            )
            db.add(mem)
            stats["processed"] += 1

        stats["duplicate_count"] = stats["duplicate_in_batch"] + stats["duplicate_existing"]
        stats["skipped"] = stats["duplicate_count"] + stats["missing_transaction_id"]

        db.commit()

        logger.info(
            "Batch processed for user %s | received=%d processed=%d skipped=%d duplicates=%d "
            "(batch=%d, existing=%d, missing_id=%d)",
            user_uuid,
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
