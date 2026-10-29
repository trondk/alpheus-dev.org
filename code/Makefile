.PHONY: all generate check-generated test test-crd vet

all: generate vet test

# Deepcopy functions and the CRD, from the kubebuilder markers in api/.
generate:
	go tool controller-gen object:headerFile=hack/boilerplate.go.txt paths=./api/...
	go tool controller-gen crd paths=./api/... output:crd:artifacts:config=config/crd/bases

# Fails when generated code is not up to date (GO-63).
check-generated: generate
	git diff --exit-code -- api config

vet:
	go vet ./...

test:
	go test -race ./...

# Checks the CRD's schema and CEL rules against the cluster in KUBECONFIG,
# for example the Vagrant test cluster (source ../vagrant/credentials.sh).
test-crd: generate
	hack/test-crd.sh
