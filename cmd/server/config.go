package main

import (
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultGameStatesSaved     = 5
	defaultGameStatesShiftBack = 1
	defaultTicksPerSecond      = 30
	defaultPlayerCount         = 2
	defaultPlayerPickUpRange   = 10
	defaultPlayerDropRange     = 15
	defaultPlayerRadius        = 5
	defaultMapFilePath         = "test/testmap.json"
	defaultPortToAcceptConns   = 9979

	defaultEnableUsersServiceUpdate       = false
	defaultUsersServiceAddress            = "https://users-service-medieval.herokuapp.com"
	defaultUsersServiceStatsEndpoint      = "/v1/stats/"
	defaultUsersServiceCurrenciesEndpoint = "/v1/users/{id}/currencies"
	defaultUsersServiceBaseCoins          = 50
	defaultUsersServiceDamageCoins        = 1.0
	defaultUsersServiceKillCoins          = 75.0
	defaultUsersServiceTimeout            = 10 * time.Second
	defaultUsersServiceToken              = ""

	defaultBackoffInitialDuration = 15 * time.Second
	defaultBackoffMaxDuration     = 45 * time.Second
	defaultBackoffFactor          = 2.0
	defaultBackoffRandomization   = 0.0
	defaultBackoffMaxInterval     = 90 * time.Second
)

var (
	flagGameStatesSaved     = pflag.Int("gamesession.states.saved", defaultGameStatesSaved, "previous game states stored")
	flagGameStatesShiftBack = pflag.Int("gamesession.states.shiftback", defaultGameStatesShiftBack, "amount of gamestates to go back, when calculating current game state")
	flagTicksPerSecond      = pflag.Int("gamesession.ticks", defaultTicksPerSecond, "server ticks per second")
	flagPlayerCount         = pflag.Int("gamesession.player.count", defaultPlayerCount, "players in the session")
	flagPlayerPickUpRange   = pflag.Float32("gamesession.player.pickup", defaultPlayerPickUpRange, "range of player item pick up")
	flagPlayerDropRange     = pflag.Float32("gamesession.player.drop", defaultPlayerDropRange, "range of player item drop")
	flagPlayerRadius        = pflag.Int("gamesession.player.radius", defaultPlayerRadius, "radius of player model")
	flagMapFilePath         = pflag.String("gamemanager.map.file", defaultMapFilePath, "path to map description")
	flagPortToAcceptConns   = pflag.Int("gameserver.port", defaultPortToAcceptConns, "port to expose to clients")

	flagUsersServiceEnabled            = pflag.Bool("users_service.enabled", defaultEnableUsersServiceUpdate, "make requests to users service")
	flagUsersServiceAddress            = pflag.String("users_service.address", defaultUsersServiceAddress, "users service address")
	flagUsersServiceTimeout            = pflag.Duration("users_service.timeout", defaultUsersServiceTimeout, "users service timeout")
	flagUsersServiceToken              = pflag.String("users_service.token", defaultUsersServiceToken, "users service s2s token")
	flagUsersServiceStatsEndpoint      = pflag.String("users_service.stats.endpoint", defaultUsersServiceStatsEndpoint, "users service stats endpoint")
	flagUsersServiceCurrenciesEndpoint = pflag.String("users_service.currencies.endpoint", defaultUsersServiceCurrenciesEndpoint, "users service currencies endpoint")
	flagUsersServiceBaseCoins          = pflag.Int("users_service.currencies.base", defaultUsersServiceBaseCoins, "users service base coins for 1 game")
	flagUsersServiceDamageCoins        = pflag.Float64("users_service.currencies.damage", defaultUsersServiceDamageCoins, "users service coins for 1 damage")
	flagUsersServiceKillCoins          = pflag.Float64("users_service.currencies.kill", defaultUsersServiceKillCoins, "users service coins for 1 kill")

	flagEnableUsersServiceUpdate = pflag.Bool("users_service.enabled", defaultEnableUsersServiceUpdate, "enable  updates")

	flagBackoffInitDuration  = pflag.Duration("backoff.init_duration", defaultBackoffInitialDuration, "initial duration of retry")
	flagBackoffMaxDuration   = pflag.Duration("backoff.max_duration", defaultBackoffMaxDuration, "initial duration of retry")
	flagBackoffFactor        = pflag.Float64("backoff.factor", defaultBackoffFactor, "the factor to multiply the previous duration by for the next step in exponential backoff")
	flagBackoffRandomization = pflag.Float64("backoff.randomization", defaultBackoffRandomization, "at each step in the retry the timeout will be between [duration, duration * randomization + duration] to prevent clients from converging on periodic behavior")
	flagBackoffMaxInterval   = pflag.Duration("backoff.max_interval", defaultBackoffMaxInterval, "time duration to retry, if duration ends, no more retries are made")
)
