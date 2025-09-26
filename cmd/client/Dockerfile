FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/client ./cmd/client

FROM gcr.io/distroless/base-debian12
ENV SERVER_ADDR=server:8080 \
    LOG_LEVEL=info
COPY --from=build /out/client /client
USER nonroot:nonroot
ENTRYPOINT ["/client"]
