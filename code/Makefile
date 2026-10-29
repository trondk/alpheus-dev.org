.PHONY: all generate check-generated test test-crd test-s3 vet

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

# Runs internal/store's tests against Garage in the Vagrant test cluster,
# over HTTPS through the gateway, trusting the test CA.
VAGRANT := $(abspath ../vagrant)
test-s3:
	. $(VAGRANT)/_out/secrets.env && \
	ALPHEUS_TEST_S3_ENDPOINT=https://$$($(VAGRANT)/talos.sh ip):30900 \
	ALPHEUS_TEST_S3_BUCKET=alpheus-data \
	ALPHEUS_TEST_S3_REGION=garage \
	ALPHEUS_TEST_S3_ACCESS_KEY=$$GARAGE_DEFAULT_ACCESS_KEY \
	ALPHEUS_TEST_S3_SECRET_KEY=$$GARAGE_DEFAULT_SECRET_KEY \
	ALPHEUS_TEST_S3_CA_FILE=$(VAGRANT)/_out/ca.crt \
	go test -race -count=1 -run TestS3 -v ./internal/store
