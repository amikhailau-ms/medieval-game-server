package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/amikhailau/medieval-game-server/pkg/connection"
	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func main() {
	absPath, _ := filepath.Abs("")
	mapPath := filepath.Join(absPath, viper.GetString("gamemanager.map.file"))
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
		panic(err)
	}

	fmt.Println("Server Initialized!")

	<-gm.FinishChan
}
