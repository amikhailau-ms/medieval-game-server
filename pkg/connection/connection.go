package connection

import (
	"context"
	"io"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

const (
	UserIDMetadata      = "User-ID"
	AuthorizationHeader = "Token"
)

type GameManagerConfig struct {
	Gscfg   *gamesession.GameSessionConfig
	MapFile string
	Uscfg   *UsersServiceConfig
}

type ClientConnection struct {
	streamServer pb.GameManager_TalkServer
	lastSeen     time.Time
	done         chan error
	playerId     int32
	userId       string
	token        string
	nickname     string
	sync.RWMutex
}

type GameManager struct {
	pb.UnimplementedGameManagerServer
	FinishChan  chan bool
	gs          *gamesession.GameSession
	cfg         *GameManagerConfig
	clients     map[string]*ClientConnection
	clientCount int
	startChan   chan bool
	gameOngoing bool
	sync.Mutex
}

func NewGameManager(cfg *GameManagerConfig) (*GameManager, error) {
	gs, err := gamesession.NewGameSession(cfg.Gscfg, cfg.MapFile)
	if err != nil {
		return nil, err
	}

	gm := &GameManager{
		cfg:         cfg,
		clients:     make(map[string]*ClientConnection),
		gs:          gs,
		clientCount: 0,
		startChan:   make(chan bool, cfg.Gscfg.PlayerCount),
		FinishChan:  make(chan bool),
	}

	go func() {
		for i := 0; i < cfg.Gscfg.PlayerCount; i++ {
			<-gm.startChan
		}

		gm.gameOngoing = true
		go gm.BroadcastNotification(&pb.ServerNotification{
			Type: pb.ServerNotificationType_GAME_STARTED,
		})

		for _, client := range gm.clients {
			gs.SetPlayerInfo(client.nickname, client.userId, client.playerId)
		}

		prevGameStates := make([]gamesession.PrevGameState, 0, cfg.Gscfg.GameStatesSaved)
		for i := 0; i < cfg.Gscfg.GameStatesSaved; i++ {
			prevGameStates = append(prevGameStates, gs.GameState.GetPrevGameState(gm.cfg.Gscfg.PlayerCount, gm.cfg.Gscfg.PlayerRadius))
		}
		gs.PrevGameStates = prevGameStates

		tickDuration := time.Millisecond * time.Duration(1000.0/float64(gm.cfg.Gscfg.TicksPerSecond))
		ticker := time.NewTicker(tickDuration)

		for {
			<-ticker.C
			endGame := gs.DoSessionTick()
			go gm.BroadcastGameState()

			moreMessages := true
			for moreMessages {
				select {
				case playerId := <-gs.AttackNotifications:
					go gm.BroadcastNotification(&pb.ServerNotification{
						Type:  pb.ServerNotificationType_PLAYER_ATTACKED,
						Actor: strconv.Itoa(int(playerId)),
					})
				default:
					moreMessages = false
				}

			}

			moreMessages = true
			for moreMessages {
				select {
				case killInfo := <-gs.KillNotifications:
					go gm.BroadcastNotification(&pb.ServerNotification{
						Type:     pb.ServerNotificationType_PLAYER_KILLED,
						Actor:    killInfo.Actor,
						Receiver: killInfo.Receiver,
					})
				default:
					moreMessages = false
				}

			}
			if endGame {
				break
			}
		}

		ticker.Stop()
		go gm.BroadcastNotification(&pb.ServerNotification{
			Type: pb.ServerNotificationType_GAME_FINISHED,
		})
		gm.SendResults()
		gm.FinishChan <- true
	}()

	return gm, nil
}

func (gm *GameManager) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	receiveTime := time.Now().UTC()

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Unable to retrieve metadata")
	}

	userId := md.Get(UserIDMetadata)
	if len(userId) < 1 {
		return nil, status.Error(codes.InvalidArgument, "No user id value set")
	}
	//CHECK THAT THIS USER BELONGS TO THIS SESSION?

	clientToken := uuid.New().String()

	clientTime := req.LocalTime.AsTime().UTC()
	if receiveTime.Sub(clientTime) > 500*time.Millisecond {
		return nil, status.Error(codes.OutOfRange, "Ping too big")
	}

	ping := int32(float64(receiveTime.Nanosecond()-clientTime.Nanosecond()) / 1000000)
	log.Printf("Player with user id %v has registered\n", userId)

	gm.Lock()
	defer gm.Unlock()

	newClient := &ClientConnection{
		lastSeen: time.Now(),
		playerId: int32(gm.clientCount),
		userId:   userId[0],
		token:    clientToken,
		done:     make(chan error),
		nickname: req.Nickname,
	}
	gm.clients[clientToken] = newClient
	gm.clientCount++

	return &pb.ConnectResponse{
		Ping:  ping,
		Token: clientToken,
	}, nil
}

func (gm *GameManager) Talk(srv pb.GameManager_TalkServer) error {
	ctx := srv.Context()

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "No token set")
	}
	tokenRaw := md.Get(AuthorizationHeader)
	if len(tokenRaw) == 0 {
		return status.Error(codes.Unauthenticated, "No token set")
	}

	_, err := uuid.Parse(tokenRaw[0])
	if err != nil {
		return status.Error(codes.Unauthenticated, "Unable to validate token")
	}

	client, found := gm.clients[tokenRaw[0]]
	if !found {
		return status.Error(codes.Unauthenticated, "Invalid token set")
	}
	client.Lock()
	client.streamServer = srv
	client.Unlock()

	go func() {
		go gm.BroadcastNotification(&pb.ServerNotification{
			Type:  pb.ServerNotificationType_PLAYER_CONNECTED,
			Actor: client.nickname,
		})
		for {
			req, err := srv.Recv()
			if err != nil {
				if err == io.EOF {
					client.done <- status.Error(codes.Aborted, "client disconnected")
				}
				client.done <- status.Error(codes.DataLoss, "unable to receive message from client")
			}
			client.lastSeen = time.Now().UTC()

			if not := req.GetNotification(); not != nil {
				switch not.Type {
				case pb.NotificationType_DISCONNECT:
					log.Printf("Player with user id %v has disconnected\n", client.userId)
					client.done <- status.Error(codes.Aborted, "client requested disconnect")
					break
				case pb.NotificationType_CONNECT:
					log.Printf("Player with user id %v has connected\n", client.userId)
					gm.startChan <- true
					continue
				}
			}

			if gm.gameOngoing {
				if action := req.GetAction(); action != nil {
					gm.gs.ProcessAction(action, client.playerId)
				}
			}
		}
	}()

	var doneErr error
	select {
	case <-ctx.Done():
		doneErr = ctx.Err()
	case doneErr = <-client.done:
	}

	client.Lock()
	client.streamServer = nil
	client.Unlock()
	go gm.BroadcastNotification(&pb.ServerNotification{
		Type:  pb.ServerNotificationType_PLAYER_DISCONNECTED,
		Actor: client.nickname,
	})
	if doneErr != nil && status.Code(doneErr) != codes.Aborted {
		return status.Error(codes.Internal, "error occured while processing actions")
	}

	return nil
}

func (gm *GameManager) BroadcastNotification(not *pb.ServerNotification) {
	serverTime := ptypes.TimestampNow()
	for _, client := range gm.clients {
		client.RLock()
		if client.streamServer != nil {
			if err := client.streamServer.Send(&pb.ServerResponse{Info: &pb.ServerResponse_Notification{Notification: not}, ServerTime: serverTime}); err != nil {
				log.Printf("user{id: %v, playerId: %v, nickname: %v} - unable to reach: %v\n", client.userId, client.playerId, client.nickname, err)
				client.done <- err
			} else {
				log.Printf("user{id: %v, playerId: %v, nickname: %v} - received notication: %v\n", client.userId, client.playerId, client.nickname, not.Type)
			}
		}
		client.RUnlock()
	}
}

func (gm *GameManager) BroadcastGameState() {
	serverTime := ptypes.TimestampNow()
	gm.gs.RLock()
	newGameState := &pb.GameState{
		Players:      gm.gs.PrevGameStates[gm.cfg.Gscfg.GameStatesSaved-gm.cfg.Gscfg.GameStatesShiftBack].Players,
		DroppedItems: gm.gs.PrevGameStates[gm.cfg.Gscfg.GameStatesSaved-gm.cfg.Gscfg.GameStatesShiftBack].Items,
		PlayersLeft:  int32(gm.gs.PrevGameStates[gm.cfg.Gscfg.GameStatesSaved-gm.cfg.Gscfg.GameStatesShiftBack].PlayersLeft),
	}
	gm.gs.RUnlock()
	for _, client := range gm.clients {
		client.RLock()
		if client.streamServer != nil {
			if err := client.streamServer.Send(&pb.ServerResponse{Info: &pb.ServerResponse_GameState{GameState: newGameState}, ServerTime: serverTime}); err != nil {
				log.Printf("user{id: %v, playerId: %v, nickname: %v} - unable to reach: %v\n", client.userId, client.playerId, client.nickname, err)
				client.done <- err
			} else {
				log.Printf("user{id: %v, playerId: %v, nickname: %v} - received game state\n", client.userId, client.playerId, client.nickname)
			}
		}
		client.RUnlock()
	}
}
