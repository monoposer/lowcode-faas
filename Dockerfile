# Build dispatcher + worker; runtime includes docker CLI for runner.RunJS.
#
# 若拉取 Docker Hub 出现 EOF / timeout，可用镜像加速，例如：
#   docker compose -f docker-compose.dev.yml build \
#     --build-arg GO_IMAGE=docker.m.daocloud.io/library/golang:1.22-bookworm \
#     --build-arg RUNTIME_IMAGE=docker.m.daocloud.io/library/debian:bookworm-slim
# 或在 .env 中设置 LOWCODE_FAAS_DOCKER_GO_IMAGE / LOWCODE_FAAS_DOCKER_RUNTIME_IMAGE（见 docker-compose.dev.yml）。
ARG GO_IMAGE=golang:1.22-bookworm
ARG RUNTIME_IMAGE=debian:bookworm-slim

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dispatcher ./cmd/dispatcher \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM ${RUNTIME_IMAGE}
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates docker.io \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/dispatcher /out/worker /usr/local/bin/
WORKDIR /app
EXPOSE 8080 9090
# Default: dispatcher; override in compose for worker.
CMD ["dispatcher"]
