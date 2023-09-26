# Copyright the peerfintech. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# Supported Targets:

# unit-test: runs all the unit tests

# Build flags (overridable)
GO_LDFLAGS                 ?=

## integration-test
GO_CMD                 ?= go
MAKEFILE_THIS          := $(lastword $(MAKEFILE_LIST))
THIS_PATH              := $(patsubst %/,%,$(dir $(abspath $(MAKEFILE_THIS))))
TEST_SCRIPTS_PATH      := test/scripts
TEST_FIXTURES_PATH     := test/fixtures
TEST_E2E_PATH          := github.com/PeerFintech/gosdk/test/integration/e2e


export GO_CMD

.PHONY: integration-test
integration-test: dockerupfabric downloadvendor setup-test

.PHONY: integration-gm-test
integration-gm-test:
	@echo "integration test for gm fabric"
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-etcdraft.yaml up -d
	go test -v  -run=TestE2EGM $(TEST_E2E_PATH)

.PHONY: integration-test-v2
integration-test-v2:
	@echo "integration-test rely on fabric-2.2.X, default image_tag:2.2.3"
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-etcdraft.yaml up -d
	go test -v  -run=TestV2E2ENonGM $(TEST_E2E_PATH)

.PHONY: integration-gm-test-v2
integration-gm-test-v2:
	@echo "integration test for gm fabric-2.2"
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-etcdraft.yaml up -d
	go test -v  -run=TestV2E2EGM $(TEST_E2E_PATH)

.PHONY: dockerupfabric
dockerupfabric:
	@echo "integration-test rely on fabric-1.4.x, default image_tag:1.4.9"
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-etcdraft.yaml up -d

.PHONY: downloadvendor
downloadvendor:
	@go mod vendor && cd $(TEST_FIXTURES_PATH)/chaincode && go mod vendor

.PHONY: setup-test
setup-test:
	go test -v  -run=TestE2ENonGM $(TEST_E2E_PATH)

.PHONY: clienthandler-test
clienthandler-test:
	go test -v  -run=TestFabricClientHandler $(TEST_E2E_PATH)

.PHONY: withoutset-test
withoutset-test:
	@echo "please make sure that integration-test operation has been performed"
	go test -v -run=TestRunWithoutSet $(TEST_E2E_PATH)

.PHONY: discover-test
discover-test:
	@echo "please make sure that integration-test operation has been performed"
	go test -v -run=TestDiscover $(TEST_E2E_PATH)


.PHONY: clean
clean: clean-integration-test

.PHONY:clean-integration-test
clean-integration-test:
	docker-compose -f $(TEST_FIXTURES_PATH)/docker-compose-etcdraft.yaml down
	-@docker ps -a|grep "dev\-peer"|awk '{print $$1}'|xargs docker rm -f
	-@docker images |grep "^dev\-peer"|awk '{print $$3}'|xargs docker rmi -f

.PHONY: unit-test
unit-test:
	@MODULE="github.com/PeerFintech/gosdk" \
	PKG_ROOT="./pkg" \
	$(TEST_SCRIPTS_PATH)/unit.sh

.PHONY: ledger-test
ledger-test:
	@echo "please make sure that integration-test operation has been performed"
	go test -v -run=TestLedgerHandler $(TEST_E2E_PATH)
