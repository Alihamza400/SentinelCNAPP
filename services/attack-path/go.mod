module github.com/sentinel-cnapp/sentinel-cnapp/services/attack-path

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/neo4j/neo4j-go-driver/v5 v5.20.0
	github.com/prometheus/client_golang v1.19.1
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
