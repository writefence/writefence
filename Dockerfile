# Stage 1: build
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /writefence ./cmd/writefence/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /writefence-cli ./cmd/writefence-cli/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /mock-memory-store ./demo/mock-memory-store.go

# Stage 2: minimal runtime
FROM scratch
COPY --from=builder /writefence /writefence
COPY --from=builder /writefence-cli /writefence-cli
COPY --from=builder /mock-memory-store /mock-memory-store
EXPOSE 9622
ENTRYPOINT ["/writefence"]
