# Py-Embed

Py-Embed is a web server for creating, storing & working with text embeddings.

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Usage](#usage)
4. [Features](#features)

## Introduction

A detailed description of the project, its purpose, and the problem it solves.

## Installation

Step-by-step instructions on how to install the project.

```bash
# Navigate to the project directory
cd py-embed

# Create a virtual environment

python -m venv venv
source venv/bin/activate

# or using conda
conda create --name venv python=3.10
conda activate venv
```

Install the required dependencies

```bash
pip install -r requirements.txt
```

## Usage

Instructions on how to use the project. Include code snippets and examples where applicable.

```bash
# Run the database
docker compose -f compose.dev.yml up -d db

# Running the main script
uvicorn app.main:app --reload
```

## Features

List of features implemented in the project.

- Embedding API
- Embeddings search

## Swagger

Swagger docs available at http://localhost:8000/docs
