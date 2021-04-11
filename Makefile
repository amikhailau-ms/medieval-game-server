include Makefile.vars

.PHONY fmt: fmt-local
fmt-local:
	@go fmt $(GO_PACKAGES)

.PHONY test: test-local
test-local: fmt-local
	@go test $(GO_TEST_FLAGS) $(GO_PACKAGES)

docker-build:
	@docker build -f $(DOCKERFILE_PATH) -t $(SERVER_IMAGE):$(IMAGE_VERSION) .
	@docker build -f $(MM_DOCKERFILE_PATH) -t $(SERVER_IMAGE):matchmaker-$(IMAGE_VERSION) .
	@docker image prune -f --filter label=stage=server-intermediate

.docker-$(IMAGE_NAME)-$(IMAGE_VERSION):
	$(MAKE) docker-build
	touch $@

.PHONY: docker
docker: .docker-$(IMAGE_NAME)-$(IMAGE_VERSION)

docker-push: docker
	@docker push $(SERVER_IMAGE):$(IMAGE_VERSION)
	@docker push $(SERVER_IMAGE):matchmaker-$(IMAGE_VERSION)

.push-$(IMAGE_NAME)-$(IMAGE_VERSION):
	$(MAKE) docker-push
	touch $@

.PHONY: push
push: .push-$(IMAGE_NAME)-$(IMAGE_VERSION)

pb-generate:
	protoc --proto_path=$(PROJECT_ROOT)/pkg/pb:${GOPATH}/src --go_out=$(PROJECT_ROOT)/pkg/pb --go-grpc_out=$(PROJECT_ROOT)/pkg/pb --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative $(PROJECT_ROOT)/pkg/pb/gameserver.proto
	$(GENERATOR) $(MM_PROTOBUF_ARGS) $(PROJECT_GOLANG_PATH)/pkg/mpb/matchmaker.proto

deploy-locally:
	minikube start --kubernetes-version v1.18.15 --vm-driver virtualbox -p game-cluster
	kubectl create namespace medieval-game-server
	helm install agones-installation --namespace agones-system --set "gameservers.namespaces={medieval-game-server}" --create-namespace agones/agones
	sleep 30
	kubectl apply -f $(PROJECT_ROOT)/deploy/server/fleet.yaml -n medieval-game-server
	kubectl apply -f $(PROJECT_ROOT)/deploy/matchmaker/deployment.yaml
	kubectl expose deployment medieval-game-server-matchmaker --type=LoadBalancer --name=matchmaker -n medieval-game-server
	minikube service matchmaker -n medieval-game-server -p game-cluster

undeploy-locally:
	minikube stop -p game-cluster
	minikube delete -p game-cluster

delete-locally:
	kubectl delete namespace medieval-game-server
	helm uninstall agones-installation --namespace=agones-system
