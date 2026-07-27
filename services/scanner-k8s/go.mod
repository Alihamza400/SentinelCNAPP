module github.com/sentinel-cnapp/sentinel-cnapp/services/scanner-k8s

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	k8s.io/client-go v0.30.0
	k8s.io/api v0.30.0
	k8s.io/apimachinery v0.30.0
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
