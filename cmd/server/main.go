package main

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/amikhailau/medieval-game-server/pkg/connection"
	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	sdk "agones.dev/agones/sdks/go"
)

func init() {
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func main() {
	doneC := make(chan error)
	absPath, _ := filepath.Abs("")
	mapPath := filepath.Join(absPath, viper.GetString("gamemanager.map.file"))

	agones, err := sdk.NewSDK()
	if err != nil {
		log.Fatalf("Could not connect to sdk: %v", err)
	}

	port := viper.GetInt("gameserver.port")
	log.Printf("listening on port %d", port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}

	gm, err := connection.NewGameManager(&connection.GameManagerConfig{
		Gscfg: &gamesession.GameSessionConfig{
			GameStatesSaved:     viper.GetInt("gamesession.states.saved"),
			GameStatesShiftBack: viper.GetInt("gamesession.states.shiftback"),
			TicksPerSecond:      viper.GetInt("gamesession.ticks"),
			PlayerCount:         viper.GetInt("gamesession.player.count"),
			PlayerPickUpRange:   float32(viper.GetFloat64("gamesession.player.pickup")),
			PlayerDropRange:     float32(viper.GetFloat64("gamesession.player.drop")),
			PlayerRadius:        float32(viper.GetFloat64("gamesession.player.radius")),
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
		Uscfg: &connection.UsersServiceConfig{
			Enabled:                viper.GetBool("users_service.enabled"),
			Address:                viper.GetString("users_service.address"),
			PutStatsEndpoint:       viper.GetString("users_service.stats.endpoint"),
			PostCurrenciesEndpoint: viper.GetString("users_service.currencies.endpoint"),
			BaseCoins:              viper.GetInt("users_service.currencies.base"),
			DamageCoinsMultiplier:  viper.GetFloat64("users_service.currencies.damage"),
			KillCoinsMultiplier:    viper.GetFloat64("users_service.currencies.kill"),
			Token:                  viper.GetString("users_service.token"),
			Timeout:                viper.GetDuration("users_service.timeout"),
			BackoffCfg: &connection.BackoffConfig{
				InitialDuration: viper.GetDuration("backoff.init_duration"),
				MaxDuration:     viper.GetDuration("backoff.max_duration"),
				Randomization:   viper.GetFloat64("backoff.randomization"),
				Factor:          viper.GetFloat64("backoff.factor"),
				MaxInterval:     viper.GetDuration("backoff.max_interval"),
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to create game manager: %v\n", err)
	}

	s := grpc.NewServer()
	pb.RegisterGameManagerServer(s, gm)

	go func() {
		doneC <- s.Serve(lis)
	}()

	stop := make(chan bool)
	go doHealth(agones, stop)
	err = agones.Ready()
	if err != nil {
		log.Fatalf("unable to send ready status: %v\n", err)
	}

	fmt.Printf("Server Initialized! Serving on %v port\n", port)

	select {
	case err = <-doneC:
		log.Fatalf("failed to serve: %v\n", err)
	case <-gm.FinishChan:
	}

	stop <- true
	err = agones.Shutdown()
	if err != nil {
		log.Fatalf("unable to send shutdown signal: %v\n", err)
	}
}

func doHealth(sdk *sdk.SDK, stop <-chan bool) {
	tick := time.Tick(3 * time.Second)
	for {
		err := sdk.Health()
		if err != nil {
			log.Fatalf("Could not send health ping, %v", err)
		}
		select {
		case <-stop:
			log.Print("Stopped health pings")
			return
		case <-tick:
		}
	}
}
