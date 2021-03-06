PROJECT_ROOT=$(PWD)

pb-generate:
	protoc --proto_path=$(PROJECT_ROOT)/pkg/pb:${GOPATH}/src --go_out=$(PROJECT_ROOT)/pkg/pb --go_opt=paths=source_relative $(PROJECT_ROOT)/pkg/pb/gameserver.proto