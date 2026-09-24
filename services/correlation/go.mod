module github.com/sentinel-cnapp/sentinel-cnapp/services/correlation

go 1.22.0

require (
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.36.0
	github.com/neo4j/neo4j-go-driver/v5 v5.20.0
	github.com/prometheus/client_golang v1.19.1
	github.com/redis/go-redis/v9 v9.5.3
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
