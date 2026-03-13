import os

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://app_worker_role:worker_password@db:5432/genie_db"
)
