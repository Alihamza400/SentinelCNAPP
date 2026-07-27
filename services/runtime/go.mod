module github.com/sentinel-cnapp/sentinel-cnapp/services/runtime

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/nats-io/nats.go v1.36.0
	github.com/prometheus/client_golang v1.19.1
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
