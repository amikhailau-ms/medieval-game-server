package main

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/amikhailau/medieval-game-server/pkg/connection"
	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
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
	})
	if err != nil {
		log.Fatalf("failed to create game manager: %v\n", err)
	}

	s := grpc.NewServer()
	pb.RegisterGameManagerServer(s, gm)

	go func() {
		doneC <- s.Serve(lis)
	}()

	fmt.Printf("Server Initialized! Serving on %v port\n", port)

	select {
	case err = <-doneC:
		log.Fatalf("failed to serve: %v\n", err)
	case <-gm.FinishChan:
	}
}
