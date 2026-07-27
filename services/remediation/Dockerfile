FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/remediation/ ./services/remediation/

WORKDIR /src/services/remediation
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/remediation .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/remediation /bin/remediation
EXPOSE 8080
ENTRYPOINT ["/bin/remediation"]
