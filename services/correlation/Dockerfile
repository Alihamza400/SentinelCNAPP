FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/correlation/ ./services/correlation/

WORKDIR /src/services/correlation
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/correlation .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/correlation /bin/correlation
EXPOSE 8080
ENTRYPOINT ["/bin/correlation"]
