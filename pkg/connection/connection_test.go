package connection

import (
	"context"
	"io"
	"math"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestConnect(t *testing.T) {
	testGM := &GameManager{
		FinishChan:  make(chan bool),
		startChan:   make(chan bool, 2),
		clients:     make(map[string]*ClientConnection),
		clientCount: 0,
	}

	okTime, _ := ptypes.TimestampProto(time.Now())
	notOkTime, _ := ptypes.TimestampProto(time.Now().Add(-1 * time.Second))

	testCases := []struct {
		name string
		md   []string
		req  *pb.ConnectRequest
		err  error
	}{
		{
			name: "valid request",
			md:   []string{UserIDMetadata, "id-1"},
			req: &pb.ConnectRequest{
				LocalTime: okTime,
			},
			err: nil,
		},
		{
			name: "invalid request - metadata",
			md:   []string{},
			req: &pb.ConnectRequest{
				LocalTime: okTime,
			},
			err: status.Error(codes.InvalidArgument, "No user id value set"),
		},
		{
			name: "invalid request - ping",
			md:   []string{UserIDMetadata, "id-1"},
			req: &pb.ConnectRequest{
				LocalTime: notOkTime,
			},
			err: status.Error(codes.OutOfRange, "Ping too big"),
		},
	}

	for _, td := range testCases {
		t.Run(td.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(td.md...))

			resp, err := testGM.Connect(ctx, td.req)
			if err != nil && td.err == nil || err == nil && td.err != nil || err != nil && td.err != nil && td.err.Error() != err.Error() {
				t.Fatalf("expected error: %v, got: %v", td.err, err)
			}

			if resp != nil {
				_, err = uuid.Parse(resp.Token)
				if err != nil {
					t.Error("expected token in response to be uuid")
				}

				if resp.Ping < 0 {
					t.Errorf("expected ping to be positive, got: %v", resp.Ping)
				}
			}
		})
	}
}

func TestTalk(t *testing.T) {
	testClients := map[string]*ClientConnection{
		"4541981a-5d78-40ac-918e-74d2d7491264": {
			nickname: "player0",
			userId:   "id-0",
			playerId: 0,
			done:     make(chan error),
		},
	}

	testGM := &GameManager{
		FinishChan:  make(chan bool),
		startChan:   make(chan bool, 5),
		clients:     testClients,
		clientCount: 0,
	}
	ctx := context.Background()

	s := grpc.NewServer()
	pb.RegisterGameManagerServer(s, testGM)

	lis, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatalf("Unable to start listening: %v", err)
	}

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Unable to serve: %v", err)
		}
	}()

	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewGameManagerClient(conn)

	testCases := []struct {
		name      string
		token     string
		reqToSend []*pb.ClientMessage
		err       error
	}{
		{
			name:  "valid requests",
			token: "4541981a-5d78-40ac-918e-74d2d7491264",
			reqToSend: []*pb.ClientMessage{
				{
					Message: &pb.ClientMessage_Notification{Notification: &pb.Notification{Type: pb.NotificationType_CONNECT}},
				},
				{
					Message: &pb.ClientMessage_Notification{Notification: &pb.Notification{Type: pb.NotificationType_DISCONNECT}},
				},
			},
			err: nil,
		},
		{
			name:      "no token",
			token:     "",
			reqToSend: []*pb.ClientMessage{},
			err:       status.Error(codes.Unauthenticated, "No token set"),
		},
		{
			name:      "invalid token",
			token:     "226690dd-97bf-42d5-bc8c-e69790fd62ae",
			reqToSend: []*pb.ClientMessage{},
			err:       status.Error(codes.Unauthenticated, "Invalid token set"),
		},
	}

	for _, td := range testCases {
		t.Run(td.name, func(t *testing.T) {
			ctx := context.Background()
			if td.token != "" {
				ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{AuthorizationHeader: td.token}))
			}

			stream, err := client.Talk(ctx)
			if err != nil {
				t.Fatalf("Error creating stream to server: %v", err)
			}

			if stream != nil {

				connErrChan := make(chan error, 10)
				sendErrChan := make(chan error, len(td.reqToSend))

				go func() {
					for {
						rec, err := stream.Recv()
						if err != nil {
							if err != io.EOF {
								connErrChan <- err
							}
							break
						} else {
							t.Logf("Received from server: %v", *rec.GetNotification())
						}
					}
				}()

				go func() {
					serverChange := false
					for _, msg := range td.reqToSend {
						err := stream.Send(msg)
						if err != nil {
							sendErrChan <- err
						}
						time.Sleep(100 * time.Millisecond)
						if testGM.clients[td.token].streamServer != nil {
							serverChange = true
						}
					}
					if !serverChange && td.err == nil {
						t.Errorf("expected server connection for %v to not be empty", td.token)
					}
				}()

				time.Sleep(500 * time.Millisecond)

				select {
				case err := <-connErrChan:
					if err != nil && td.err == nil || err == nil && td.err != nil || err != nil && td.err != nil && td.err.Error() != err.Error() {
						t.Errorf("expected error: %v, got: %v", td.err, err)
					}
				case err := <-sendErrChan:
					t.Errorf("Unexpected error on send: %v", err)
				default:
				}
			}
		})
	}
}

func TestGameplayLoop(t *testing.T) {

	usersServiceAddress := "users-service:8080"
	usersServicePutEndpoint := "/v1/stats/"
	usersServicePostEndpoint := "/v1/users/{id}/currencies"
	usersServiceToken := "some-admin-token"

	absPath, _ := filepath.Abs("")
	mapPath := filepath.Join(absPath[:len(absPath)-14], "/test/testmap.json")

	gm, err := NewGameManager(&GameManagerConfig{
		Gscfg: &gamesession.GameSessionConfig{
			GameStatesSaved:     3,
			GameStatesShiftBack: 1,
			TicksPerSecond:      30,
			PlayerCount:         2,
			PlayerPickUpRange:   10,
			PlayerDropRange:     12,
			PlayerRadius:        5,
			DefaultWeapon: &pb.EquipmentItem{
				Type:   pb.EquipmentItemType_WEAPON,
				Rarity: pb.EquipmentItemRarity_DEFAULT,
				Characteristics: &pb.EquipmentItem_WeaponChars{
					WeaponChars: &pb.WeaponCharacteristics{
						AttackPower:    10,
						KnockbackPower: 2,
						Range:          7,
						AttackCone:     0.79,
					},
				},
			},
		},
		MapFile: mapPath,
		Uscfg: &UsersServiceConfig{
			Enabled:                false,
			Address:                usersServiceAddress,
			PutStatsEndpoint:       usersServicePutEndpoint,
			PostCurrenciesEndpoint: usersServicePostEndpoint,
			BaseCoins:              50,
			DamageCoinsMultiplier:  1.0,
			KillCoinsMultiplier:    75.0,
			Token:                  usersServiceToken,
			Timeout:                10 * time.Second,
			BackoffCfg: &BackoffConfig{
				InitialDuration: 2 * time.Second,
				MaxDuration:     8 * time.Second,
				Randomization:   0.0,
				Factor:          2.0,
				MaxInterval:     10 * time.Second,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create game manager: %v\n", err)
	}

	s := grpc.NewServer()
	pb.RegisterGameManagerServer(s, gm)

	lis, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatalf("Unable to start listening: %v", err)
	}

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Unable to serve: %v", err)
		}
	}()

	t.Log("Server started")

	userIDs := []string{"user-id-1", "user-id-2"}
	clientMessages := []struct {
		messages []*pb.ClientMessage
	}{
		{
			messages: []*pb.ClientMessage{
				{
					Message: &pb.ClientMessage_Notification{Notification: &pb.Notification{Type: pb.NotificationType_CONNECT}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 10, Y: 20},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 20, Y: 20},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: 2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
			},
		},
		{
			messages: []*pb.ClientMessage{
				{
					Message: &pb.ClientMessage_Notification{Notification: &pb.Notification{Type: pb.NotificationType_CONNECT}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: -30, Y: 0},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: -20, Y: -30},
						Angle: math.Pi,
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Move{Move: &pb.MovementAction{
						Shift: &pb.Vector{X: 0, Y: -2},
					}}}},
				},
				{
					Message: &pb.ClientMessage_Action{Action: &pb.Action{Action: &pb.Action_Attack{}}},
				},
			},
		},
	}

	var userWG sync.WaitGroup

	for i, userID := range userIDs {

		userWG.Add(1)

		go func(userID string, index int) {
			t.Logf("Initiating user %v", userID)

			ctx := context.Background()

			conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				t.Errorf("User %v unable to establish connection: %v", userID, err)
			}
			defer conn.Close()

			t.Logf("User %v established connection", userID)

			client := pb.NewGameManagerClient(conn)

			connectCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(UserIDMetadata, userID))
			resp, err := client.Connect(connectCtx, &pb.ConnectRequest{
				LocalTime: ptypes.TimestampNow(),
				Nickname:  "nick-" + userID,
			})
			if err != nil {
				t.Errorf("User %v unable to connect to the server: %v", userID, err)
			}

			t.Logf("User %v connected to the server, ping %v", userID, resp.Ping)

			talkCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(AuthorizationHeader, resp.Token))
			talkClient, err := client.Talk(talkCtx)
			if err != nil {
				t.Errorf("User %v unable to talk to the server: %v", userID, err)
			}

			t.Logf("User %v started talking to server", userID)

			connErrChan := make(chan error, 10)
			sendErrChan := make(chan error, len(clientMessages[index].messages)*2)
			finishedSending := make(chan bool)

			go func() {
				for {
					rec, err := talkClient.Recv()
					select {
					case <-finishedSending:
						break
					default:
					}
					if err != nil {
						if err != io.EOF {
							connErrChan <- err
						}
						break
					} else {
						if gs := rec.GetGameState(); gs != nil {
							playerHP := 100
							position := &pb.Vector{}
							for _, player := range gs.Players {
								if player.UserId == userID {
									playerHP = int(player.Hp)
									position = player.Position
								}
							}
							t.Logf("User %v received state from server, hp %v, position %v %v", userID, playerHP, position.X, position.Y)
						}
						if not := rec.GetNotification(); not != nil {
							t.Logf("User %v received notification from server: %v", userID, not)
						}
					}
				}
			}()

			go func() {
				for _, msg := range clientMessages[index].messages {
					err := talkClient.Send(msg)
					if err != nil {
						sendErrChan <- err
					}
					time.Sleep(50 * time.Millisecond)
				}
				talkClient.CloseSend()
				finishedSending <- true
			}()

			looping := true
			for looping {
				select {
				case err := <-connErrChan:
					t.Errorf("unexpected connection error for user %v: %v", userID, err)
				case err := <-sendErrChan:
					t.Errorf("unexpected send error for user %v: %v", userID, err)
				default:
					select {
					case <-finishedSending:
						looping = false
					default:
					}
				}
			}

			userWG.Done()
		}(userID, i)

		time.Sleep(10 * time.Millisecond)
	}

	userWG.Wait()
	<-gm.FinishChan
}
