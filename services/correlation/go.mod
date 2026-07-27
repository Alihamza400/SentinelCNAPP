module github.com/sentinel-cnapp/sentinel-cnapp/services/correlation

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/nats-io/nats.go v1.36.0
	github.com/neo4j/neo4j-go-driver/v5 v5.20.0
	github.com/redis/go-redis/v9 v9.5.3
	github.com/prometheus/client_golang v1.19.1
	github.com/google/uuid v1.6.0
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
