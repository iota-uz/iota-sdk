FROM python:3.10 as install

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
WORKDIR /

FROM install as prod
CMD uvicorn app.main:app --host 0.0.0.0 --port 8000

FROM install as dev
CMD uvicorn app.main:app --reload --port 8000 --host 0.0.0.0
