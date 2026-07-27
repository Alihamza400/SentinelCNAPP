FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/asset-inventory/ ./services/asset-inventory/

WORKDIR /src/services/asset-inventory
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/asset-inventory .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/asset-inventory /bin/asset-inventory
EXPOSE 8080
ENTRYPOINT ["/bin/asset-inventory"]
