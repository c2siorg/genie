import uuid
from sqlalchemy import Column, String, Float, DateTime, ForeignKey, Text, Index
from sqlalchemy.dialects.postgresql import UUID
from pgvector.sqlalchemy import Vector

from app.db.session import Base

class Transaction(Base):
    __tablename__ = "transactions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    transaction_id = Column(String, unique=True, nullable=False)
    amount = Column(Float)
    merchant = Column(String)
    date = Column(DateTime)


class SemanticMemory(Base):
    __tablename__ = "semantic_memories"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    transaction_id = Column(String, ForeignKey("transactions.transaction_id", ondelete="CASCADE"))
    content = Column(Text)
    embedding = Column(Vector(384))

    __table_args__ = (
        Index(
            "ix_semantic_memories_embedding_hnsw",
            "embedding",
            postgresql_using="hnsw",
            postgresql_with={"m": 16, "ef_construction": 64},
            postgresql_ops={"embedding": "vector_cosine_ops"}
        ),
    )
