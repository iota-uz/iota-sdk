import typing

from fastapi import APIRouter
from pydantic import BaseModel

from app.internal.model import model, get_device, new_text_splitter

router = APIRouter(prefix="/encode")


class EncodeQueryBody(BaseModel):
    query: str


class EncodeBody(BaseModel):
    text: str
    batch_size: typing.Optional[int] = 32
    chunk_size: typing.Optional[int] = 1000


class BulkEncodeItem(BaseModel):
    id: str
    text: str


class BulkEncodeBody(BaseModel):
    texts: list[BulkEncodeItem]
    batch_size: typing.Optional[int] = 32
    chunk_size: typing.Optional[int] = 1000


class EmbeddingBody(BaseModel):
    text: str
    reference_id: str
    meta: typing.Optional[dict] = None
    language: typing.Optional[str] = None
    batch_size: typing.Optional[int] = 32
    chunk_size: typing.Optional[int] = 1000


class EncodeOutput(BaseModel):
    text: str
    embedding: list[float]


class BulkEncodeOutput(BaseModel):
    id: str
    text: str
    embedding: list[float]


@router.post("/query", description="Encode query into embeddings array without breaking up the text")
def encode_query(data: EncodeQueryBody):
    return model.encode(
        data.query,
        batch_size=data.batch_size,
        device=get_device(),
        show_progress_bar=False
    ).tolist()


@router.post("/", description="Break into chunks & encode")
def encode_text_corpus(data: EncodeBody) -> list[EncodeOutput]:
    text_splitter = new_text_splitter(data.chunk_size, 20)
    documents = text_splitter.split_text(data.text)
    output: list[list[float]] = model.encode(
        documents,
        batch_size=data.batch_size,
        device=get_device(),
        show_progress_bar=False
    ).tolist()
    return [EncodeOutput(text=text, embedding=e) for text, e in zip(documents, output)]


@router.post("/bulk")
def bulk_encode(data: BulkEncodeBody) -> list[BulkEncodeOutput]:
    text_splitter = new_text_splitter(data.chunk_size, 20)
    input_data: list[dict] = []
    for item in data.texts:
        for text in text_splitter.split_text(item.text):
            input_data.append({"id": item.id, "text": text})
    raw_input = [item["text"] for item in input_data]
    embeddings_list = model.encode(
        raw_input,
        batch_size=data.batch_size,
        device=get_device(),
        show_progress_bar=True
    )
    output: list[BulkEncodeOutput] = []
    for i, item in enumerate(input_data):
        output.append(BulkEncodeOutput(id=item["id"], text=item["text"], embedding=embeddings_list[i].tolist()))
    return output
