FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY services/runtime/ ./services/runtime/

WORKDIR /src/services/runtime
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/runtime .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/runtime /bin/runtime
EXPOSE 8080
ENTRYPOINT ["/bin/runtime"]
