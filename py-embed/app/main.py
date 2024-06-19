import json
import resource
import typing

from app.internal.db import conn
from app.internal.model import model, get_device, new_text_splitter
from app.routers import encode
from fastapi import FastAPI
from pydantic import BaseModel

resource.setrlimit(resource.RLIMIT_NOFILE, (65536, 65536))
app = FastAPI()
app.include_router(encode.router)


class SearchEmbeddingsBody(BaseModel):
    query: str
    language: typing.Optional[str] = None
    cutoff: typing.Optional[float] = 0.2
    top_k: typing.Optional[int] = 10


class SearchResult(BaseModel):
    uuid: str
    text: str
    reference_id: str
    score: float


class CreateEmbeddingsBody(BaseModel):
    text: str
    reference_id: str
    meta: dict
    language: typing.Optional[str] = None
    batch_size: typing.Optional[int] = 32
    chunk_size: typing.Optional[int] = 1000


class EmbeddingsOutputItem(BaseModel):
    uuid: str
    text: str
    embedding: list[float]


class CreateEmbeddingsOutput(BaseModel):
    items: list[EmbeddingsOutputItem]


@app.post("/embeddings/search", description="Search for embeddings")
def search_embeddings(data: SearchEmbeddingsBody) -> list[SearchResult]:
    embedding = model.encode(
        data.query,
        batch_size=32,
        device=get_device(),
        show_progress_bar=False
    ).tolist()
    sql_query = """SELECT * FROM (SELECT uuid, text, reference_id, 1 - cosine_distance(embedding, (%s)) AS score
            FROM embeddings) as matches WHERE score > %s ORDER BY score DESC LIMIT %s;"""
    cursor = conn.cursor()
    cursor.execute(sql_query, (str(embedding), data.cutoff, data.top_k))
    rows = cursor.fetchall()
    return [SearchResult(uuid=row[0], text=row[1], reference_id=row[2], score=row[3]) for row in rows]


@app.post("/embeddings", description="Create embedding record")
def create_embeddings(data: CreateEmbeddingsBody) -> CreateEmbeddingsOutput:
    text_splitter = new_text_splitter(data.chunk_size, 20)
    documents = text_splitter.split_text(data.text)
    output: list[list[float]] = model.encode(
        documents,
        batch_size=data.batch_size,
        device=get_device(),
        show_progress_bar=False
    ).tolist()
    result: list[EmbeddingsOutputItem] = []
    for text, e in zip(documents, output):
        cursor = conn.cursor()
        cursor.execute(
            """INSERT INTO embeddings (text, reference_id, embedding, language, meta) 
            VALUES (%s, %s, %s, %s, %s) RETURNING uuid;""",
            (text, data.reference_id, str(e), data.language, json.dumps(data.meta))
        )
        uuid = cursor.fetchone()[0]
        result.append(EmbeddingsOutputItem(uuid=uuid, text=text, embedding=e))
        conn.commit()
    return CreateEmbeddingsOutput(items=result)


@app.delete("/embeddings/{uuid}", description="Delete embedding record by uuid")
def delete_embeddings(uuid: str):
    return {"uuid": uuid}


@app.delete("/embeddings/reference/{reference_id}", description="Delete record by reference_id")
def delete_embeddings(reference_id: str):
    return {"reference_id": reference_id}
