include Makefile.vars

.PHONY fmt: fmt-local
fmt-local:
	@go fmt $(GO_PACKAGES)

.PHONY test: test-local
test-local: fmt-local
	@go test $(GO_TEST_FLAGS) $(GO_PACKAGES)

docker-build:
	@docker build -f $(DOCKERFILE_PATH) -t $(SERVER_IMAGE):$(IMAGE_VERSION) .
	@docker image prune -f --filter label=stage=server-intermediate

.docker-$(IMAGE_NAME)-$(IMAGE_VERSION):
	$(MAKE) docker-build
	touch $@

.PHONY: docker
docker: .docker-$(IMAGE_NAME)-$(IMAGE_VERSION)

docker-push:
	@docker push $(SERVER_IMAGE):$(IMAGE_VERSION)

.push-$(IMAGE_NAME)-$(IMAGE_VERSION):
	$(MAKE) docker-push
	touch $@

.PHONY: push
push: .push-$(IMAGE_NAME)-$(IMAGE_VERSION)

pb-generate:
	protoc --proto_path=$(PROJECT_ROOT)/pkg/pb:${GOPATH}/src --go_out=$(PROJECT_ROOT)/pkg/pb --go-grpc_out=$(PROJECT_ROOT)/pkg/pb --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative $(PROJECT_ROOT)/pkg/pb/gameserver.proto