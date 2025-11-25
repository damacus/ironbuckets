FROM golang:1.25-alpine
ARG MINIO_VERSION=latest
ARG MINIO_ROOT_USER=minioadmin
ARG MINIO_ROOT_PASSWORD=minioadmin

RUN go install github.com/minio/minio@${MINIO_VERSION} && \
    go install github.com/minio/mc@latest && \
    adduser -D -u 1000 minio && \
    mkdir -p /data && \
    chown -R minio:minio /data

ENV MC_HOST_local=http://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@localhost:9000

USER minio
EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["minio"]
CMD ["server", "/data", "--console-address", ":9001"]
