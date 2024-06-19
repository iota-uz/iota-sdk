import os

import psycopg2

conn = psycopg2.connect(
    database=os.environ.get("DB_NAME", "embeddings_db"),
    host=os.environ.get("DB_HOST", "localhost"),
    user=os.environ.get("DB_USER", "postgres"),
    password=os.environ.get("DB_PASSWORD", "postgres"),
    port=os.environ.get("DB_PORT", "5433")
)
