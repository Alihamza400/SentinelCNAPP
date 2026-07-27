FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/ai-assistant/ ./services/ai-assistant/

WORKDIR /src/services/ai-assistant
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ai-assistant .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/ai-assistant /bin/ai-assistant
EXPOSE 8080
ENTRYPOINT ["/bin/ai-assistant"]
