package main

import "github.com/spf13/pflag"

const (
	defaultGameStatesSaved     = 10
	defaultGameStatesShiftBack = 5
	defaultTicksPerSecond      = 30
	defaultPlayerCount         = 2
	defaultPlayerPickUpRange   = 10
	defaultPlayerDropRange     = 15
	defaultPlayerRadius        = 5
	defaultMapFilePath         = "test/testmap.json"
	defaultPortToAcceptConns   = 9979
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
)
