.PHONY: all generate proto check-generated check-deps test test-crd test-s3 test-agent-s3 vet

all: generate vet check-deps test

# Deepcopy functions and the CRD from the kubebuilder markers in api/, and
# the controller's RBAC role from those in internal/controller.
generate:
	go tool controller-gen object:headerFile=hack/boilerplate.go.txt paths=./api/...
	go tool controller-gen crd paths=./api/... output:crd:artifacts:config=config/crd/bases
	go tool controller-gen rbac:roleName=alpheus-controller paths=./internal/controller/... output:rbac:artifacts:config=config/rbac
	$(MAKE) proto

# Go code for proto/ in internal/pb, with buf and the generators in go.mod.
proto:
	buf generate

# Fails when generated code is not up to date (GO-63).
check-generated: generate
	git diff --exit-code -- api config internal/pb

vet:
	go vet ./...

# The controller's tests run against kube-apiserver and etcd from envtest,
# which setup-envtest downloads the first time.
ENVTEST_K8S := 1.37.0
test:
	KUBEBUILDER_ASSETS="$$(go tool setup-envtest use $(ENVTEST_K8S) -p path)" go test -race ./...

# Checks the CRD's schema and CEL rules against the cluster in KUBECONFIG,
# for example the Vagrant test cluster (source vagrant/credentials.sh).
test-crd: generate
	hack/test-crd.sh

# Runs internal/store's tests against Garage in the Vagrant test cluster,
# over HTTPS through the gateway, trusting the test CA.
VAGRANT := $(abspath vagrant)
GARAGE_ENV = . $(VAGRANT)/_out/secrets.env && \
	ALPHEUS_TEST_S3_ENDPOINT=https://$$($(VAGRANT)/talos.sh ip):30900 \
	ALPHEUS_TEST_S3_BUCKET=alpheus-data \
	ALPHEUS_TEST_S3_REGION=garage \
	ALPHEUS_TEST_S3_ACCESS_KEY=$$GARAGE_DEFAULT_ACCESS_KEY \
	ALPHEUS_TEST_S3_SECRET_KEY=$$GARAGE_DEFAULT_SECRET_KEY \
	ALPHEUS_TEST_S3_CA_FILE=$(VAGRANT)/_out/ca.crt
test-s3:
	$(GARAGE_ENV) go test -race -count=1 -run TestS3 -v ./internal/store

# Runs agents and components in-process against Garage, as testcase03's
# pipeline with signals through the builtin hub and a restart.
test-agent-s3:
	$(GARAGE_ENV) go test -race -count=1 -run TestS3 -v ./internal/agent

# Alpheus never reads a chunk's content, so it must not depend on Parquet or
# Arrow (GO-101).
check-deps:
	@if go list -deps ./... | grep -q '^github.com/apache/arrow-go'; then \
		echo "arrow-go is a dependency (GO-101)"; exit 1; fi
