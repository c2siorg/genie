from fastapi import FastAPI
from contextlib import asynccontextmanager
from app.api.endpoints import router as transactions_router

@asynccontextmanager
async def lifespan(app: FastAPI):
    yield

app = FastAPI(lifespan=lifespan, title="Genie API")
app.include_router(transactions_router, prefix="/api/v1/transactions")
