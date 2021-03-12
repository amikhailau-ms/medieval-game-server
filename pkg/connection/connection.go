package connection

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
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
	GameManager pb.UnimplementedGameManagerServer
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

	userIdInterface := ctx.Value("User-ID")
	if userIdInterface == nil {
		return nil, status.Error(codes.Unauthenticated, "No user id set")
	}

	userId, ok := userIdInterface.(string)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Incorrect user id value")
	}
	//CHECK THAT THIS USER BELONGS TO THIS SESSION?

	h := sha512.New()
	h.Write([]byte(userId + time.Now().String()))
	clientToken := hex.EncodeToString(h.Sum(nil))

	clientTime, err := ptypes.Timestamp(req.LocalTime)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Incorrect local time value")
	}

	if receiveTime.Second()-clientTime.Second() > 1 {
		return nil, status.Error(codes.OutOfRange, "Ping too big")
	}

	ping := int32(float64(receiveTime.Nanosecond()-clientTime.Nanosecond()) / 1000000)

	gm.Lock()
	defer gm.Unlock()

	newClient := &ClientConnection{
		lastSeen: receiveTime,
		playerId: int32(gm.clientCount),
		userId:   userId,
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

//WHAT ABOUT THE OTHER ONE?
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
			if not.Type == pb.NotificationType_DISCONNECT {
				client.done <- status.Error(codes.Aborted, "client requested disconnect")
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
