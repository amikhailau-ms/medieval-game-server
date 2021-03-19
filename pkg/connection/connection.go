package connection

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

const (
	UserIDMetadata = "User-ID"
)

type GameManagerConfig struct {
	Gscfg   *gamesession.GameSessionConfig
	MapFile string
}

type ClientConnection struct {
	streamServer pb.GameManager_TalkServer
	lastSeen     time.Time
	done         chan error
	playerId     int32
	userId       string
	token        string
	nickname     string
}

type GameManager struct {
	pb.UnimplementedGameManagerServer
	FinishChan  chan bool
	gs          *gamesession.GameSession
	cfg         *GameManagerConfig
	clients     map[string]*ClientConnection
	clientCount int
	startChan   chan bool
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

		for _, client := range gm.clients {
			gs.SetPlayerInfo(client.nickname, client.userId, client.playerId)
		}

		prevGameStates := make([]gamesession.PrevGameState, 0, cfg.Gscfg.GameStatesSaved)
		for i := 0; i < cfg.Gscfg.GameStatesSaved; i++ {
			prevGameStates = append(prevGameStates, gs.GameState.GetPrevGameState(gm.cfg.Gscfg.PlayerCount, gm.cfg.Gscfg.PlayerRadius))
		}
		gs.PrevGameStates = prevGameStates

		for {
			endGame := gs.DoSessionTick()
			go gm.BroadcastGameState()
			if endGame {
				break
			}
		}

		gm.SendResults()
		gm.FinishChan <- true
	}()

	return gm, nil
}

func (gm *GameManager) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	receiveTime := time.Now()

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Unable to retrieve metadata")
	}

	userId := md.Get(UserIDMetadata)
	if len(userId) < 1 {
		return nil, status.Error(codes.InvalidArgument, "No user id value set")
	}
	//CHECK THAT THIS USER BELONGS TO THIS SESSION?

	h := sha512.New()
	h.Write([]byte(userId[0] + time.Now().String()))
	clientToken := hex.EncodeToString(h.Sum(nil))

	clientTime, err := ptypes.Timestamp(req.LocalTime)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Incorrect local time value")
	}

	if receiveTime.Second()-clientTime.Second() > 1 {
		return nil, status.Error(codes.OutOfRange, "Ping too big")
	}

	ping := int32(float64(receiveTime.Nanosecond()-clientTime.Nanosecond()) / 1000000)
	log.Printf("Player with user id %v has registered\n", userId)

	gm.Lock()
	defer gm.Unlock()

	newClient := &ClientConnection{
		lastSeen: receiveTime,
		playerId: int32(gm.clientCount),
		userId:   userId[0],
		token:    clientToken,
		done:     make(chan error),
	}
	gm.clients[clientToken] = newClient

	return &pb.ConnectResponse{
		Ping:       ping,
		Token:      clientToken,
		ServerTime: ptypes.TimestampNow(),
	}, nil
}

func (gm *GameManager) Talk(srv pb.GameManager_TalkServer) error {
	ctx := srv.Context()

	clientTokenInterface := ctx.Value("Token")
	if clientTokenInterface == nil {
		return status.Error(codes.Unauthenticated, "No token set")
	}

	clientToken, ok := clientTokenInterface.(string)
	if !ok {
		return status.Error(codes.Unauthenticated, "Invalid token set")
	}

	client, found := gm.clients[clientToken]
	if !found {
		return status.Error(codes.Unauthenticated, "Invalid token set")
	}
	client.streamServer = srv

	go func() {
		req, err := srv.Recv()
		if err != nil {
			client.done <- status.Error(codes.DataLoss, "unable to receive message from client")
		}

		if not := req.GetNotification(); not != nil {
			switch not.Type {
			case pb.NotificationType_DISCONNECT:
				log.Printf("Player with user id %v has disconnected\n", client.userId)
				go gm.BroadcastNotification(&pb.ServerNotification{
					Type:  pb.ServerNotificationType_PLAYER_DISCONNECTED,
					Actor: client.nickname,
				})
				client.done <- status.Error(codes.Aborted, "client requested disconnect")
			case pb.NotificationType_CONNECT:
				log.Printf("Player with user id %v has connected\n", client.userId)
				go gm.BroadcastNotification(&pb.ServerNotification{
					Type:  pb.ServerNotificationType_PLAYER_CONNECTED,
					Actor: client.nickname,
				})
			}
		}

		if action := req.GetAction(); action != nil {
			gm.gs.ProcessAction(action, client.playerId)
		}
	}()

	gm.startChan <- true

	var doneErr error
	select {
	case <-ctx.Done():
		doneErr = ctx.Err()
	case doneErr = <-client.done:
	}
	if doneErr != nil {
		return status.Error(codes.Internal, "error occured while processing actions")
	}

	client.streamServer = nil
	return nil
}

func (gm *GameManager) SendResults() {
	//NEED TO ADD STATS TO THE GAME
}

func (gm *GameManager) BroadcastNotification(not *pb.ServerNotification) {
	for _, client := range gm.clients {
		if client.streamServer == nil {
			continue
		}
		if err := client.streamServer.Send(&pb.ServerResponse{Info: &pb.ServerResponse_Notification{Notification: not}}); err != nil {
			log.Printf("user{id: %v, playerId: %v, nickname: %v} - unable to reach: %v\n", client.userId, client.playerId, client.nickname, err)
			client.done <- err
			continue
		}
		log.Printf("user{id: %v, playerId: %v, nickname: %v} - received notication: %v\n", client.userId, client.playerId, client.nickname, &not)
	}
}

func (gm *GameManager) BroadcastGameState() {
	newGameState := &pb.GameState{
		Players:      gm.gs.PrevGameStates[gm.cfg.Gscfg.GameStatesSaved-gm.cfg.Gscfg.GameStatesShiftBack+1].Players,
		DroppedItems: gm.gs.PrevGameStates[gm.cfg.Gscfg.GameStatesSaved-gm.cfg.Gscfg.GameStatesShiftBack+1].Items,
	}
	for _, client := range gm.clients {
		if client.streamServer == nil {
			continue
		}
		if err := client.streamServer.Send(&pb.ServerResponse{Info: &pb.ServerResponse_GameState{GameState: newGameState}}); err != nil {
			log.Printf("user{id: %v, playerId: %v, nickname: %v} - unable to reach: %v\n", client.userId, client.playerId, client.nickname, err)
			client.done <- err
			continue
		}
		log.Printf("user{id: %v, playerId: %v, nickname: %v} - received game state\n", client.userId, client.playerId, client.nickname)
	}
}
