FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

# Install Trivy
RUN wget -q https://github.com/aquasecurity/trivy/releases/download/v0.51.0/trivy_0.51.0_Linux-64bit.tar.gz && \
    tar xzf trivy_0.51.0_Linux-64bit.tar.gz trivy && \
    mv trivy /usr/local/bin/trivy && \
    rm trivy_0.51.0_Linux-64bit.tar.gz

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/scanner-container/ ./services/scanner-container/

WORKDIR /src/services/scanner-container
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scanner-container .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/scanner-container /bin/scanner-container
COPY --from=builder /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=builder /root/.cache/trivy /root/.cache/trivy
EXPOSE 8080
ENTRYPOINT ["/bin/scanner-container"]
