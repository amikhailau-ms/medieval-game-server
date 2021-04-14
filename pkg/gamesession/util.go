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
	absPath, _ := filepath.Abs("")
	filePath := filepath.Join(absPath[:len(absPath)-16], "/test/testmap.json")
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
	sortedEntities := make([]SortedEntity, 0, len(mapDesc.Polygons))
	unmovableEntities := make([]collision2d.Polygon, 0, len(mapDesc.Polygons))
	for i, polygon := range mapDesc.Polygons {
		newEntity := collision2d.NewPolygon(collision2d.NewVector(0, 0), collision2d.NewVector(0, 0), 0, polygon.Vertexes)
		box := newEntity.GetAABB()
		sortedEntities = append(sortedEntities, SortedEntity{entityId: i, value: float32(box.Pos.X + box.Points[0].X), start: true},
			SortedEntity{entityId: i, value: float32(box.Pos.X + box.Points[2].X), start: true})
		unmovableEntities = append(unmovableEntities, newEntity)
	}
	sort.SliceStable(sortedEntities, func(i, j int) bool { return sortedEntities[i].value < sortedEntities[j].value })
	items := make([]*SyncItem, 0, 6)
	helmet := &SyncItem{
		ItemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_HELMET,
				Rarity:          pb.EquipmentItemRarity_UNCOMMON,
				Characteristics: &pb.EquipmentItem_HpBuff{HpBuff: 20},
				ItemId:          0,
			},
			Position: &pb.Vector{X: mapDesc.LootSpawns[0], Y: mapDesc.LootSpawns[1]},
		},
	}
	armor := &SyncItem{
		ItemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_ARMOR,
				Rarity:          pb.EquipmentItemRarity_RARE,
				Characteristics: &pb.EquipmentItem_DamageReduction{DamageReduction: 20},
				ItemId:          1,
			},
			Position: &pb.Vector{X: mapDesc.LootSpawns[2], Y: mapDesc.LootSpawns[3]},
		},
	}
	helmetEnemy := &SyncItem{
		ItemInfo: &pb.DroppedEquipmentItem{
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
		ItemInfo: &pb.DroppedEquipmentItem{
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
		ItemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:   pb.EquipmentItemType_WEAPON,
				Rarity: pb.EquipmentItemRarity_COMMON,
				Characteristics: &pb.EquipmentItem_WeaponChars{WeaponChars: &pb.WeaponCharacteristics{
					AttackPower:    20,
					KnockbackPower: 3,
					Range:          15,
					AttackCone:     0.79,
				}},
				ItemId: 4,
			},
			Position: &pb.Vector{X: mapDesc.LootSpawns[4], Y: mapDesc.LootSpawns[5]},
		},
	}
	helmet2 := &SyncItem{
		ItemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_HELMET,
				Rarity:          pb.EquipmentItemRarity_RARE,
				Characteristics: &pb.EquipmentItem_HpBuff{HpBuff: 30},
				ItemId:          5,
			},
			Position: &pb.Vector{X: mapDesc.LootSpawns[6], Y: mapDesc.LootSpawns[7]},
		},
	}
	items = append(items, helmet, armor, helmetEnemy, armorEnemy, weapon, helmet2)
	defaultWeapon := &pb.EquipmentItem{
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
	}
	players := make([]*SyncPlayer, 0, 4)
	player := &SyncPlayer{
		PlayerInfo: &pb.Player{
			Nickname:  "player",
			Hp:        100,
			UserId:    "some-id",
			Position:  &pb.Vector{X: 50, Y: 40},
			Angle:     math.Pi / 2,
			PlayerId:  0,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy()},
			Stats:     &pb.PlayerStats{},
		},
		Position: 0,
	}
	enemy1 := &SyncPlayer{
		PlayerInfo: &pb.Player{
			Nickname:  "enemy1",
			Hp:        100,
			UserId:    "some-id-1",
			Position:  &pb.Vector{X: 80, Y: 20},
			Angle:     math.Pi / 2,
			PlayerId:  1,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy()},
			Stats:     &pb.PlayerStats{},
		},
		Position: 0,
	}
	enemy2 := &SyncPlayer{
		PlayerInfo: &pb.Player{
			Nickname:  "enemy2",
			Hp:        80,
			UserId:    "some-id-2",
			Position:  &pb.Vector{X: 70, Y: 80},
			Angle:     0,
			PlayerId:  2,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy(), Helmet: helmetEnemy.ItemInfo.Item},
			Stats:     &pb.PlayerStats{},
		},
		Position: 0,
	}
	enemy3 := &SyncPlayer{
		PlayerInfo: &pb.Player{
			Nickname:  "enemy3",
			Hp:        70,
			UserId:    "some-id-3",
			Position:  &pb.Vector{X: 80, Y: 80},
			Angle:     math.Pi * 3 / 2,
			PlayerId:  3,
			Equipment: &pb.PlayerEquipment{Weapon: defaultWeapon.Deepcopy(), Armor: armorEnemy.ItemInfo.Item},
			Stats:     &pb.PlayerStats{},
		},
		Position: 0,
	}
	players = append(players, player, enemy1, enemy2, enemy3)
	currentGameState := CurrentGameState{
		Players:     players,
		Items:       items,
		PlayersLeft: 4,
	}
	prevGameStates := make([]PrevGameState, 0, 10)
	for i := 0; i < 10; i++ {
		prevGameStates = append(prevGameStates, currentGameState.GetPrevGameState(4, 5))
	}
	gameSession := &GameSession{
		unmovableEntities: unmovableEntities,
		sortedEntities:    sortedEntities,
		cfg: &GameSessionConfig{
			GameStatesSaved:     10,
			GameStatesShiftBack: 1,
			TicksPerSecond:      30,
			PlayerCount:         4,
			PlayerPickUpRange:   10,
			PlayerDropRange:     12,
			PlayerRadius:        5,
			DefaultWeapon:       defaultWeapon,
		},
		GameState:           currentGameState,
		PrevGameStates:      prevGameStates,
		mapBorderX:          mapDesc.MapBorderX,
		mapBorderY:          mapDesc.MapBorderY,
		AttackNotifications: make(chan int32, 10),
		KillNotifications:   make(chan KillInfo, 5),
		deadPlayers:         make(chan int32, 5),
	}
	return gameSession, nil
}

func (x *CurrentGameState) GetPrevGameState(PlayerCount int, PlayerRadius float32) PrevGameState {
	sortedPlayers := make([]SortedPlayer, 0, PlayerCount)
	players := make([]*pb.Player, 0, PlayerCount)
	items := make([]*pb.DroppedEquipmentItem, 0, PlayerCount)
	for _, player := range x.Players {
		minGotYou := player.PlayerInfo.Position.X - PlayerRadius
		maxGotYou := player.PlayerInfo.Position.X + PlayerRadius
		sortedPlayers = append(sortedPlayers, SortedPlayer{playerId: player.PlayerInfo.PlayerId, value: minGotYou, start: true},
			SortedPlayer{playerId: player.PlayerInfo.PlayerId, value: maxGotYou, start: false})
		players = append(players, player.PlayerInfo.Deepcopy())
	}
	sort.SliceStable(sortedPlayers, func(i, j int) bool { return sortedPlayers[i].value < sortedPlayers[j].value })

	for _, item := range x.Items {
		items = append(items, item.ItemInfo.Deepcopy())
	}

	newPrevGameState := PrevGameState{SortedPlayers: sortedPlayers, Players: players, Items: items}
	return newPrevGameState
}

func SectorCollision(minAngleSec1, maxAngleSec1, minAngleSec2, maxAngleSec2 float32) bool {
	switch {
	case minAngleSec1 > maxAngleSec1 && minAngleSec2 > maxAngleSec2:
		return true
	case minAngleSec1 > maxAngleSec1 && minAngleSec2 <= maxAngleSec2:
		return minAngleSec1 < minAngleSec2 || minAngleSec1 < maxAngleSec2 || maxAngleSec1 > minAngleSec2 || maxAngleSec1 > maxAngleSec2
	case minAngleSec1 <= maxAngleSec1 && minAngleSec2 > maxAngleSec2:
		return minAngleSec2 < minAngleSec1 || minAngleSec2 < maxAngleSec1 || maxAngleSec2 > minAngleSec1 || maxAngleSec2 > maxAngleSec1
	case minAngleSec1 <= maxAngleSec1 && minAngleSec2 <= maxAngleSec2:
		return minAngleSec1 < minAngleSec2 && maxAngleSec1 > maxAngleSec2 || maxAngleSec2 > minAngleSec1 && minAngleSec1 > minAngleSec2 ||
			maxAngleSec2 > maxAngleSec1 && maxAngleSec1 > minAngleSec2 || maxAngleSec1 > minAngleSec2 && minAngleSec2 > minAngleSec1 ||
			maxAngleSec1 > maxAngleSec2 && minAngleSec2 > maxAngleSec1
	}
	return false
}
