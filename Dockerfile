FROM golang:1.24 AS build
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/server ./cmd/server

FROM gcr.io/distroless/base-debian12
ENV LISTEN_ADDR=:8080 POW_DIFFICULTY=22 POW_TTL=60s LOG_LEVEL=info
COPY --from=build /bin/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
