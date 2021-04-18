package main

import (
	"time"

	"github.com/spf13/pflag"
)

const (
	// configuration defaults support local development (i.e. "go run ...")

	// Server
	defaultServerAddress = "0.0.0.0"
	defaultServerPort    = "9090"

	// Gateway
	defaultGatewayEnable      = true
	defaultGatewayAddress     = "0.0.0.0"
	defaultGatewayPort        = "8080"
	defaultGatewayURL         = "/"
	defaultGatewaySwaggerFile = "pkg/pb/matchmaker.swagger.json"

	// Health
	defaultInternalEnable    = false
	defaultInternalAddress   = "0.0.0.0"
	defaultInternalPort      = "9081"
	defaultInternalHealth    = "/healthz"
	defaultInternalReadiness = "/ready"

	defaultConfigDirectory = "deploy/"
	defaultConfigFile      = ""
	defaultSecretFile      = ""
	defaultApplicationID   = "medieval-game-server-matchmaker"

	// Heartbeat
	defaultKeepaliveTime    = 10
	defaultKeepaliveTimeout = 20

	// Logging
	defaultLoggingLevel = "debug"

	//Service specific values
	defaultAgonesEnabled   = false
	defaultLobbySize       = 2
	defaultLobbyDelay      = 2 * time.Second
	defaultMatchExpiration = 2 * time.Minute
	defaultMatchCleanup    = 5 * time.Minute
)

var (
	// define flag overrides
	flagServerAddress = pflag.String("matchmaker.server.address", defaultServerAddress, "adress of gRPC server")
	flagServerPort    = pflag.String("matchmaker.server.port", defaultServerPort, "port of gRPC server")

	flagGatewayAddress     = pflag.String("matchmaker.gateway.address", defaultGatewayAddress, "address of gateway server")
	flagGatewayPort        = pflag.String("matchmaker.gateway.port", defaultGatewayPort, "port of gateway server")
	flagGatewayURL         = pflag.String("matchmaker.gateway.endpoint", defaultGatewayURL, "endpoint of gateway server")
	flagGatewaySwaggerFile = pflag.String("matchmaker.gateway.swaggerFile", defaultGatewaySwaggerFile, "directory of swagger.json file")

	flagInternalEnable    = pflag.Bool("matchmaker.internal.enable", defaultInternalEnable, "enable internal http server")
	flagInternalAddress   = pflag.String("matchmaker.internal.address", defaultInternalAddress, "address of internal http server")
	flagInternalPort      = pflag.String("matchmaker.internal.port", defaultInternalPort, "port of internal http server")
	flagInternalHealth    = pflag.String("matchmaker.internal.health", defaultInternalHealth, "endpoint for health checks")
	flagInternalReadiness = pflag.String("matchmaker.internal.readiness", defaultInternalReadiness, "endpoint for readiness checks")

	flagConfigDirectory = pflag.String("matchmaker.config.source", defaultConfigDirectory, "directory of the configuration file")
	flagConfigFile      = pflag.String("matchmaker.config.file", defaultConfigFile, "directory of the configuration file")
	flagSecretFile      = pflag.String("matchmaker.config.secret.file", defaultSecretFile, "directory of the secrets configuration file")
	flagApplicationID   = pflag.String("matchmaker.app.id", defaultApplicationID, "identifier for the application")

	flagKeepaliveTime    = pflag.Int("matchmaker.config.keepalive.time", defaultKeepaliveTime, "default value, in seconds, of the keepalive time")
	flagKeepaliveTimeout = pflag.Int("matchmaker.config.keepalive.timeout", defaultKeepaliveTimeout, "default value, in seconds, of the keepalive timeout")

	flagLoggingLevel = pflag.String("matchmaker.logging.level", defaultLoggingLevel, "log level of application")

	flagAgonesEnabled   = pflag.Bool("matchmaker.agones.enabled", defaultAgonesEnabled, "connect to agones")
	flagLobbySize       = pflag.Int("matchmaker.lobby.size", defaultLobbySize, "lobby size to create")
	flagLobbyDelay      = pflag.Duration("matchmaker.lobby.delay", defaultLobbyDelay, "delay between attempts to create lobby")
	flagMatchExpiration = pflag.Duration("matchmaker.match.expiration", defaultMatchExpiration, "how long to keep match allocation")
	flagMatchCleanup    = pflag.Duration("matchmaker.match.cleanup", defaultMatchCleanup, "delay between cleaning up expired match allocations")
)
