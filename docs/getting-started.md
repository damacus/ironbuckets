# Getting Started

## Prerequisites

- Go 1.25 or later
- A MinIO server (or compatible S3 endpoint)

## Installation

### From Source

```bash
git clone https://github.com/damacus/ironbuckets.git
cd ironbuckets
go build -o ironbuckets ./cmd/server
```

### Using Docker

```bash
docker pull ghcr.io/damacus/ironbuckets:latest
docker run -p 8080:8080 \
  -e MINIO_ENDPOINT=your-minio:9000 \
  ghcr.io/damacus/ironbuckets:latest
```

## Configuration

Copy the example environment file and configure your MinIO connection:

```bash
cp .env.example .env
```

Edit `.env` with your MinIO settings:

```bash
MINIO_ENDPOINT=localhost:9000
```

## Running

```bash
./ironbuckets
```

The web UI will be available at [http://localhost:8080](http://localhost:8080).

Log in using your MinIO access credentials.
