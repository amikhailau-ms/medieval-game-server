package main

import (
	"context"
	"fmt"
	"log"

	"github.com/amikhailau/medieval-game-server/pkg/allocation"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	UserIDHeader = "User-ID"
	TestUserID   = "some-user-id"
)

func main() {

	agonesClient, err := allocation.ConnectToAgonesLocal()
	if err != nil {
		log.Fatalf("unable to connect to agones: %v", err)
	}

	gsa, err := allocation.AllocateGameServer(agonesClient)
	if err != nil {
		log.Fatalf("unable to allocate server to test: %v", err)
	}

	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", gsa.Status.Address, gsa.Status.Ports[0].Port), opts...)
	if err != nil {
		log.Fatalf("unable to connect server to test: %v", err)
	}

	client := pb.NewGameManagerClient(conn)
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(UserIDHeader, TestUserID),
	)
	resp, err := client.Connect(ctx, &pb.ConnectRequest{
		UserId:    TestUserID,
		LocalTime: ptypes.TimestampNow(),
	})
	if err != nil {
		log.Fatalf("unable to send connect request server to test: %v", err)
	}

	fmt.Printf("Response from server:\n\tClientToken: %v\n\tPing: %v\n", resp.Token, resp.Ping)
}
