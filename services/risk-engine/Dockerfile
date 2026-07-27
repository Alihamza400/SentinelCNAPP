FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/risk-engine/ ./services/risk-engine/

WORKDIR /src/services/risk-engine
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/risk-engine .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/risk-engine /bin/risk-engine
EXPOSE 8080
ENTRYPOINT ["/bin/risk-engine"]
