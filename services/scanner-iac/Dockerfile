FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates && \
    pip3 install checkov --quiet 2>/dev/null || true

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/scanner-iac/ ./services/scanner-iac/

WORKDIR /src/services/scanner-iac
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scanner-iac .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/scanner-iac /bin/scanner-iac
COPY --from=builder /usr/lib/python3*/site-packages/checkov /usr/lib/checkov
COPY --from=builder /usr/local/bin/checkov /usr/local/bin/checkov
EXPOSE 8080
ENTRYPOINT ["/bin/scanner-iac"]
