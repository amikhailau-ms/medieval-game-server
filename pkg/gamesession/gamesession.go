package gamesession

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"sync"

	"github.com/Tarliton/collision2d"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

type GameSessionConfig struct {
	GameStatesSaved     int
	GameStatesShiftBack int
	TicksPerSecond      int
	PlayerCount         int
	PlayerPickUpRange   float32
	PlayerDropRange     float32
	PlayerRadius        float32
	DefaultWeapon       *pb.EquipmentItem
}

type GameSession struct {
	sync.RWMutex
	unmovableEntities []collision2d.Polygon
	sortedEntities    []SortedEntity
	PrevGameStates    []PrevGameState
	GameState         CurrentGameState
	cfg               *GameSessionConfig
	mapBorderX        float32
	mapBorderY        float32
	MapDesc           MapDescription
}

type SortedEntity struct {
	entityId int
	value    float32
	start    bool
}

type SortedPlayer struct {
	playerId int32
	value    float32
	start    bool
}

type PrevGameState struct {
	SortedPlayers []SortedPlayer
	Players       []*pb.Player
	Items         []*pb.DroppedEquipmentItem
}

type CurrentGameState struct {
	Players []*SyncPlayer
	Items   []*SyncItem
}

type SyncPlayer struct {
	PlayerInfo *pb.Player
	sync.Mutex
}

type SyncItem struct {
	ItemInfo *pb.DroppedEquipmentItem
	pickedUp bool
	sync.Mutex
}

type PolygonJSON struct {
	Vertexes []float64 `json:"vertexes"`
}

type MapDescription struct {
	Polygons     []PolygonJSON `json:"entities"`
	LootSpawns   []float32     `json:"loot_spots"`
	PlayerSpawns []float32     `json:"player_spawns"`
	MapBorderX   float32       `json:"map_border_x"`
	MapBorderY   float32       `json:"map_border_y"`
}

func NewGameSession(cfg *GameSessionConfig, mapFilename string) (*GameSession, error) {
	mapFile, err := os.Open(mapFilename)
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
		sortedEntities = append(sortedEntities, SortedEntity{entityId: i, value: float32(box.Points[0].X), start: true},
			SortedEntity{entityId: i, value: float32(box.Points[2].X), start: true})
		unmovableEntities = append(unmovableEntities, newEntity)
	}
	sort.SliceStable(sortedEntities, func(i, j int) bool { return sortedEntities[i].value < sortedEntities[j].value })
	//ITEM GENERATION?! - loot spots from mapDesc?
	amountOfItemsToSpawn := int(float64(len(mapDesc.LootSpawns)) / 2.0)
	items := make([]*SyncItem, 0, amountOfItemsToSpawn)
	for i := 0; i < amountOfItemsToSpawn; i++ {
		weaponChars := &pb.WeaponCharacteristics{
			AttackPower:    15,
			Range:          10,
			AttackCone:     math.Pi / 6,
			KnockbackPower: 3,
		}
		items = append(items, &SyncItem{ItemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_WEAPON,
				Rarity:          pb.EquipmentItemRarity_COMMON,
				Characteristics: &pb.EquipmentItem_WeaponChars{WeaponChars: weaponChars}},
			Position: &pb.Vector{X: mapDesc.LootSpawns[i*2], Y: mapDesc.LootSpawns[i*2+1]},
		}})
	}
	if cfg.PlayerCount > int(float64(len(mapDesc.PlayerSpawns))/2.0) {
		return nil, fmt.Errorf("there should be enough spawns for players")
	}
	players := make([]*SyncPlayer, 0, cfg.PlayerCount)
	for i := 0; i < cfg.PlayerCount; i++ {
		player := &SyncPlayer{
			PlayerInfo: &pb.Player{
				Hp:        100,
				Equipment: &pb.PlayerEquipment{Weapon: cfg.DefaultWeapon.Deepcopy()},
				Position:  &pb.Vector{X: mapDesc.PlayerSpawns[i*2], Y: mapDesc.PlayerSpawns[i*2+1]},
				Angle:     math.Pi / 2,
				PlayerId:  int32(i),
			},
		}
		players = append(players, player)
	}

	gameSession := &GameSession{
		GameState:         CurrentGameState{Items: items, Players: players},
		cfg:               cfg,
		unmovableEntities: unmovableEntities,
		sortedEntities:    sortedEntities,
		mapBorderX:        mapDesc.MapBorderX,
		mapBorderY:        mapDesc.MapBorderY,
	}
	return gameSession, nil
}

func (gs *GameSession) SetPlayerInfo(nickname, userId string, playerId int32) {
	gs.GameState.Players[int(playerId)].PlayerInfo.Nickname = nickname
	gs.GameState.Players[int(playerId)].PlayerInfo.UserId = userId
	gs.GameState.Players[int(playerId)].PlayerInfo.PlayerId = playerId
}
