package gamesession

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/Tarliton/collision2d"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func CalculateDistance(x1, y1, x2, y2 float32) float32 {
	return float32(math.Sqrt(math.Pow(float64(x1-x2), 2) + math.Pow(float64(y1-y2), 2)))
}

func MakeTestGameSession() (*GameSession, error) {
	filePath, _ := filepath.Abs("../test/testmap.json")
	mapFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("Error opening map file: %v", err)
	}
	defer mapFile.Close()
	bytes, _ := ioutil.ReadAll(mapFile)
	var mapDesc MapDescription
	err = json.Unmarshal(bytes, &mapDesc)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling map file: %v", err)
	}
	sortedEntities := make([]SortedEntity, 0, len(mapDesc.polygons))
	unmovableEntities := make([]collision2d.Polygon, 0, len(mapDesc.polygons))
	for i, polygon := range mapDesc.polygons {
		newEntity := collision2d.NewPolygon(collision2d.NewVector(0, 0), collision2d.NewVector(0, 0), 0, polygon.vertexes)
		box := newEntity.GetAABB()
		sortedEntities = append(sortedEntities, SortedEntity{entityId: i, value: float32(box.Points[0].X), start: true},
			SortedEntity{entityId: i, value: float32(box.Points[2].X), start: true})
		unmovableEntities[i] = newEntity
	}
	sort.SliceStable(sortedEntities, func(i, j int) bool { return sortedEntities[i].value < sortedEntities[j].value })
	items := make([]*SyncItem, 0, 5)
	helmet := &SyncItem{
		itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_HELMET,
				Rarity:          pb.EquipmentItemRarity_UNCOMMON,
				Characteristics: &pb.EquipmentItem_HpBuff{HpBuff: 20},
				ItemId:          0,
			},
			Position: &pb.Vector{X: mapDesc.lootSpawns[0], Y: mapDesc.lootSpawns[1]},
		},
	}
	armor := &SyncItem{
		itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_ARMOR,
				Rarity:          pb.EquipmentItemRarity_RARE,
				Characteristics: &pb.EquipmentItem_DamageReduction{DamageReduction: 20},
				ItemId:          1,
			},
			Position: &pb.Vector{X: mapDesc.lootSpawns[2], Y: mapDesc.lootSpawns[3]},
		},
	}
	helmetEnemy := &SyncItem{
		itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_HELMET,
				Rarity:          pb.EquipmentItemRarity_RARE,
				Characteristics: &pb.EquipmentItem_HpBuff{HpBuff: 30},
				ItemId:          2,
			},
			Position: &pb.Vector{X: -100, Y: -100},
		},
		pickedUp: true,
	}
	armorEnemy := &SyncItem{
		itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_ARMOR,
				Rarity:          pb.EquipmentItemRarity_UNCOMMON,
				Characteristics: &pb.EquipmentItem_DamageReduction{DamageReduction: 15},
				ItemId:          3,
			},
			Position: &pb.Vector{X: -100, Y: -100},
		},
		pickedUp: true,
	}
	weapon := &SyncItem{
		itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:   pb.EquipmentItemType_WEAPON,
				Rarity: pb.EquipmentItemRarity_COMMON,
				Characteristics: &pb.EquipmentItem_WeaponChars{WeaponChars: &pb.WeaponCharacteristics{
					AttackPower:    20,
					KnockbackPower: 3,
					Range:          15,
					AttackCone:     0.44,
				}},
				ItemId: 4,
			},
			Position: &pb.Vector{X: mapDesc.lootSpawns[4], Y: mapDesc.lootSpawns[5]},
		},
	}
	items = append(items, helmet, armor, helmetEnemy, armorEnemy, weapon)
	defaultWeapon := &pb.EquipmentItem{
		Type:   pb.EquipmentItemType_WEAPON,
		Rarity: pb.EquipmentItemRarity_DEFAULT,
		Characteristics: &pb.EquipmentItem_WeaponChars{
			WeaponChars: &pb.WeaponCharacteristics{
				AttackPower:    10,
				KnockbackPower: 2,
				Range:          5,
				AttackCone:     0.79,
			},
		},
	}
	players := make([]*SyncPlayer, 0, 4)
	player := &SyncPlayer{
		playerInfo: &pb.Player{
			Nickname:  "player",
			Hp:        100,
			UserId:    "some-id",
			Position:  &pb.Vector{X: 50, Y: 40},
			Angle:     0,
			PlayerId:  0,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy()},
		},
	}
	enemy1 := &SyncPlayer{
		playerInfo: &pb.Player{
			Nickname:  "enemy1",
			Hp:        100,
			UserId:    "some-id-1",
			Position:  &pb.Vector{X: 80, Y: 20},
			Angle:     math.Pi / 2,
			PlayerId:  1,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy()},
		},
	}
	enemy2 := &SyncPlayer{
		playerInfo: &pb.Player{
			Nickname:  "enemy2",
			Hp:        80,
			UserId:    "some-id-2",
			Position:  &pb.Vector{X: 80, Y: 20},
			Angle:     0,
			PlayerId:  2,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy(), Helmet: helmetEnemy.itemInfo.Item},
		},
	}
	enemy3 := &SyncPlayer{
		playerInfo: &pb.Player{
			Nickname:  "enemy3",
			Hp:        70,
			UserId:    "some-id-3",
			Position:  &pb.Vector{X: 80, Y: 20},
			Angle:     math.Pi * 3 / 2,
			PlayerId:  3,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy(), Armor: armorEnemy.itemInfo.Item},
		},
	}
	players = append(players, player, enemy1, enemy2, enemy3)
	gameSession := &GameSession{
		unmovableEntities: unmovableEntities,
		sortedEntities:    sortedEntities,
		cfg: &GameSessionConfig{
			GameStatesSaved:     10,
			GameStatesShiftBack: 5,
			TicksPerSecond:      30,
			PlayerCount:         4,
			PlayerPickUpRange:   5,
			PlayerDropRange:     7,
			PlayerRadius:        5,
			DefaultWeapon:       defaultWeapon,
		},
	}
	return gameSession, nil
}

func (x *CurrentGameState) GetPrevGameState(PlayerCount int, PlayerRadius float32) *PrevGameState {
	sortedPlayers := make([]SortedPlayer, 0, PlayerCount)
	players := make([]*pb.Player, 0, PlayerCount)
	items := make([]*pb.DroppedEquipmentItem, 0, PlayerCount)
	for _, player := range x.players {
		minGotYou := player.playerInfo.Position.X - PlayerRadius
		maxGotYou := player.playerInfo.Position.X + PlayerRadius
		sortedPlayers = append(sortedPlayers, SortedPlayer{playerId: player.playerInfo.PlayerId, value: minGotYou, start: true},
			SortedPlayer{playerId: player.playerInfo.PlayerId, value: maxGotYou, start: false})
		players = append(players, player.playerInfo.Deepcopy())
	}
	sort.SliceStable(sortedPlayers, func(i, j int) bool { return sortedPlayers[i].value < sortedPlayers[j].value })

	for _, item := range x.items {
		items = append(items, item.itemInfo.Deepcopy())
	}

	newPrevGameState := PrevGameState{sortedPlayers: sortedPlayers, players: players, items: items}
	return &newPrevGameState
}
