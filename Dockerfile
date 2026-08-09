# Meta control plane only (Action CRUD + compile). Worker is an embeddable SDK — not in this image.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/lowcode-faas-meta ./cmd/meta

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/lowcode-faas-meta /lowcode-faas-meta
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/lowcode-faas-meta"]
