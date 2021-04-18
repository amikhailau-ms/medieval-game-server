package main

import (
	"time"

	"agones.dev/agones/pkg/client/clientset/versioned"
	"github.com/amikhailau/medieval-game-server/pkg/allocation"
	"github.com/amikhailau/medieval-game-server/pkg/auth"
	"github.com/amikhailau/medieval-game-server/pkg/matchmaker"
	"github.com/amikhailau/medieval-game-server/pkg/mpb"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/infobloxopen/atlas-app-toolkit/requestid"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func NewGRPCServer(logger *logrus.Logger) (*grpc.Server, error) {
	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				Time:    time.Duration(viper.GetInt("matchmaker.config.keepalive.time")) * time.Second,
				Timeout: time.Duration(viper.GetInt("matchmaker.config.keepalive.timeout")) * time.Second,
			},
		),
		grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				// logging middleware
				grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logger)),

				auth.UnaryServerInterceptor(),

				// Request-Id interceptor
				requestid.UnaryServerInterceptor(),
			),
		),
	)
	logger.Infof("Matchmaker configuration - size: %v; delay: %v; keep: %v; agones.enabled: %v.",
		viper.GetInt("matchmaker.lobby.size"), viper.GetDuration("matchmaker.lobby.delay"),
		viper.GetDuration("matchmaker.match.expiration"), viper.GetBool("matchmaker.agones.enabled"))

	var agonesClient *versioned.Clientset
	var err error
	if viper.GetBool("matchmaker.agones.enabled") {
		agonesClient, err = allocation.ConnectToAgonesInCluster()
		if err != nil {
			return nil, err
		}
		logger.Info("Successfully started agones client.")
	}

	cache := cache.New(viper.GetDuration("matchmaker.match.expiration"), viper.GetDuration("matchmaker.match.cleanup"))

	matchmakerServer := matchmaker.NewMatchmakerServer(logger, cache, &matchmaker.MatchmakerServerConfig{
		LobbySize:        viper.GetInt("matchmaker.lobby.size"),
		MatchmakingDelay: viper.GetDuration("matchmaker.lobby.delay"),
		MatchKeep:        viper.GetDuration("matchmaker.match.expiration"),
		AgonesClient:     agonesClient,
	})
	mpb.RegisterMatchmakerServer(grpcServer, matchmakerServer)

	return grpcServer, nil
}
