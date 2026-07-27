FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

# Install Gitleaks
RUN wget -q https://github.com/gitleaks/gitleaks/releases/download/v8.18.0/gitleaks_8.18.0_linux_x64.tar.gz && \
    tar xzf gitleaks_8.18.0_linux_x64.tar.gz gitleaks && \
    mv gitleaks /usr/local/bin/gitleaks && \
    rm gitleaks_8.18.0_linux_x64.tar.gz

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/scanner-secrets/ ./services/scanner-secrets/

WORKDIR /src/services/scanner-secrets
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scanner-secrets .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/scanner-secrets /bin/scanner-secrets
COPY --from=builder /usr/local/bin/gitleaks /usr/local/bin/gitleaks
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
EXPOSE 8080
ENTRYPOINT ["/bin/scanner-secrets"]
