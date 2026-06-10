FROM alpine:3.24
ARG TARGETARCH

RUN apk add --no-cache ca-certificates wget && \
    case "${TARGETARCH}" in amd64|arm64) ;; *) echo "unsupported architecture: ${TARGETARCH}" && exit 1 ;; esac && \
    wget -q -O /usr/local/bin/minio "https://dl.min.io/server/minio/release/linux-${TARGETARCH}/minio" && \
    wget -q -O /usr/local/bin/mc "https://dl.min.io/client/mc/release/linux-${TARGETARCH}/mc" && \
    chmod +x /usr/local/bin/minio /usr/local/bin/mc && \
    adduser -D -u 1000 minio && \
    mkdir -p /data && \
    chown -R minio:minio /data


USER minio
EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["minio"]
CMD ["server", "/data", "--console-address", ":9001"]
