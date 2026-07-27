FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/attack-path/ ./services/attack-path/

WORKDIR /src/services/attack-path
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/attack-path .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/attack-path /bin/attack-path
EXPOSE 8080
ENTRYPOINT ["/bin/attack-path"]
