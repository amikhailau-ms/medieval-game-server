package matchmaker

import (
	"context"
	"sync"
	"time"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	"agones.dev/agones/pkg/client/clientset/versioned"
	"github.com/amikhailau/medieval-game-server/pkg/allocation"
	"github.com/amikhailau/medieval-game-server/pkg/auth"
	"github.com/amikhailau/medieval-game-server/pkg/mpb"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

const (
	UserPrefix  = "user|"
	MatchPrefix = "match|"
)

type MatchmakerServerConfig struct {
	LobbySize        int
	MatchmakingDelay time.Duration
	AgonesClient     *versioned.Clientset
	MatchKeep        time.Duration
}

type PlayerData struct {
	UserId string
}

type MatchData struct {
	IP      string
	Port    int32
	Players []*PlayerData
}

type MatchmakerServer struct {
	sync.RWMutex
	mpb.MatchmakerServer
	playersInQueue    []*PlayerData
	playersMatchmaked map[string]bool
	matchData         *cache.Cache
	cfg               *MatchmakerServerConfig
}

var _ mpb.MatchmakerServer = &MatchmakerServer{}

func NewMatchmakerServer(logger *logrus.Logger, cache *cache.Cache, cfg *MatchmakerServerConfig) *MatchmakerServer {
	ms := &MatchmakerServer{
		playersInQueue:    make([]*PlayerData, 0),
		playersMatchmaked: make(map[string]bool),
		matchData:         cache,
		cfg:               cfg,
	}
	go ms.createMatches(logger)

	return ms
}

func (s *MatchmakerServer) Matchmake(ctx context.Context, req *mpb.MatchmakeRequest) (*mpb.MatchmakeResponse, error) {
	claims, _ := auth.GetAuthorizationData(ctx)
	logger := ctxlogrus.Extract(ctx).WithField("user_id", claims.UserId)

	logger.Info("Request to matchmake")

	s.RLock()
	_, found := s.playersMatchmaked[claims.UserId]
	s.RUnlock()
	if found {
		logger.Info("Found player matchmake request")
		return &mpb.MatchmakeResponse{}, nil
	}

	s.Lock()
	defer s.Unlock()
	s.playersInQueue = append(s.playersInQueue, &PlayerData{UserId: claims.UserId})
	s.playersMatchmaked[claims.UserId] = true

	return &mpb.MatchmakeResponse{}, nil
}

func (s *MatchmakerServer) CheckMatchmakeStatus(ctx context.Context, req *mpb.CheckMatchmakeStatusRequest) (*mpb.CheckMatchmakeStatusResponse, error) {
	claims, _ := auth.GetAuthorizationData(ctx)
	logger := ctxlogrus.Extract(ctx).WithField("user_id", claims.UserId)

	logger.Info("Request to check matchmaking state")

	s.RLock()
	_, found := s.playersMatchmaked[claims.UserId]
	s.RUnlock()
	if !found {
		logger.Info("Player matchmake request not found")
		return &mpb.CheckMatchmakeStatusResponse{
			Ready:         false,
			NotMatchmaked: true,
		}, nil
	}

	matchId, found := s.matchData.Get(UserPrefix + claims.UserId)
	if !found {
		logger.Info("Player has not been matchmaked yet")
		return &mpb.CheckMatchmakeStatusResponse{
			Ready: false,
		}, nil
	}

	matchInfo, found := s.matchData.Get(matchId.(string))
	if !found {
		logger.Info("Failed matchmaking")
		return &mpb.CheckMatchmakeStatusResponse{
			Ready:  false,
			Failed: true,
		}, nil
	}

	trMatchInfo := matchInfo.(*MatchData)

	s.Lock()
	delete(s.playersMatchmaked, claims.UserId)
	s.Unlock()

	return &mpb.CheckMatchmakeStatusResponse{
		Ready:         true,
		Ip:            trMatchInfo.IP,
		Port:          trMatchInfo.Port,
		NotMatchmaked: false,
		Failed:        false,
	}, nil
}

func (s *MatchmakerServer) CancelMatchmake(ctx context.Context, req *mpb.CancelMatchmakeRequest) (*mpb.CancelMatchmakeResponse, error) {
	claims, _ := auth.GetAuthorizationData(ctx)
	logger := ctxlogrus.Extract(ctx).WithField("user_id", claims.UserId)

	logger.Info("Request to cancel matchmaking")

	s.RLock()
	_, found := s.playersMatchmaked[claims.UserId]
	s.RUnlock()
	if !found {
		logger.Info("Could not find player matchmake request")
		return &mpb.CancelMatchmakeResponse{}, nil
	}

	s.Lock()
	defer s.Unlock()
	for i := range s.playersInQueue {
		if s.playersInQueue[i].UserId == claims.UserId {
			s.playersInQueue = append(s.playersInQueue[:i], s.playersInQueue[i+1:]...)
			delete(s.playersMatchmaked, claims.UserId)
			return &mpb.CancelMatchmakeResponse{}, nil
		}
	}

	return &mpb.CancelMatchmakeResponse{}, nil
}

func (s *MatchmakerServer) createMatches(logger *logrus.Logger) {
	ticker := time.NewTicker(s.cfg.MatchmakingDelay)

	for {
		s.checkIfLobbyFits(logger)
		<-ticker.C
	}
}

func (s *MatchmakerServer) checkIfLobbyFits(logger *logrus.Logger) {

	s.RLock()
	readySize := len(s.playersInQueue)
	s.RUnlock()

	logger.Infof("Matchmaking, queue length %d", readySize)
	var err error

	if readySize >= s.cfg.LobbySize {
		s.Lock()
		playersInLobby := s.playersInQueue[:s.cfg.LobbySize]
		s.playersInQueue = s.playersInQueue[s.cfg.LobbySize:]
		s.Unlock()

		alloc := &allocationv1.GameServerAllocation{
			Status: allocationv1.GameServerAllocationStatus{
				Address: "127.0.0.1",
				Ports:   []agonesv1.GameServerStatusPort{{Port: 12345}},
			},
		}
		if s.cfg.AgonesClient != nil {
			alloc, err = allocation.AllocateGameServer(s.cfg.AgonesClient)
			if err != nil {
				logger.Errorf("Allocation of game server failed: %v", err)
				for _, player := range playersInLobby {
					s.matchData.Set(UserPrefix+player.UserId, "no-match-happened", s.cfg.MatchKeep)
				}
				return
			}
		}

		matchData := &MatchData{
			Players: playersInLobby,
			IP:      alloc.Status.Address,
			Port:    alloc.Status.Ports[0].Port,
		}
		matchId := MatchPrefix + uuid.New().String()
		s.matchData.Set(matchId, matchData, s.cfg.MatchKeep)

		for _, player := range playersInLobby {
			s.matchData.Set(UserPrefix+player.UserId, matchId, s.cfg.MatchKeep)
		}
	}
}
