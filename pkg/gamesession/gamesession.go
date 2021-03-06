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
	PlayerDropRange     float64
	PlayerRadius        float32
	DefaultWeapon       *pb.EquipmentItem
}

type GameSession struct {
	sync.RWMutex
	unmovableEntities []collision2d.Polygon
	sortedEntities    []SortedEntity
	prevGameStates    []PrevGameState
	gameState         CurrentGameState
	cfg               *GameSessionConfig
	mapBorderX        float32
	mapBorderY        float32
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
	sortedPlayers []SortedPlayer
	players       []*pb.Player
	items         []*pb.DroppedEquipmentItem
}

type CurrentGameState struct {
	players []*SyncPlayer
	items   []*SyncItem
}

type SyncPlayer struct {
	playerInfo *pb.Player
	sync.Mutex
}

type SyncItem struct {
	itemInfo *pb.DroppedEquipmentItem
	pickedUp bool
	sync.Mutex
}

type PolygonJSON struct {
	vertexes []float64 `json:"vertexes"`
}

type MapDescription struct {
	polygons   []PolygonJSON `json:"entities"`
	lootSpawns []float32     `json:"loot_spots"`
	mapBorderX float32       `json:"map_border_x"`
	mapBorderY float32       `json:"map_border_y"`
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
	//ITEM GENERATION?! - loot spots from mapDesc?
	items := make([]*SyncItem, 0, 5)
	for i := 0; i < 5; i++ {
		weaponChars := &pb.WeaponCharacteristics{
			AttackPower:    5,
			Range:          5,
			AttackCone:     math.Pi / 6,
			KnockbackPower: 3,
		}
		items[i] = &SyncItem{itemInfo: &pb.DroppedEquipmentItem{
			Item: &pb.EquipmentItem{
				Type:            pb.EquipmentItemType_WEAPON,
				Rarity:          pb.EquipmentItemRarity_COMMON,
				Characteristics: &pb.EquipmentItem_WeaponChars{WeaponChars: weaponChars}},
			Position: &pb.Vector{X: float32(i) * 20, Y: float32(i) * 20},
		}}
	}
	gameSession := &GameSession{
		gameState:         CurrentGameState{items: items},
		cfg:               cfg,
		unmovableEntities: unmovableEntities,
		sortedEntities:    sortedEntities,
		mapBorderX:        mapDesc.mapBorderX,
		mapBorderY:        mapDesc.mapBorderY,
	}
	return gameSession, nil
}
