package gamesession

import (
	"sort"

	"github.com/Tarliton/collision2d"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func (g *GameSession) DoSessionTick() bool {
	g.Lock()
	defer g.Unlock()

	sortedPlayers := make([]SortedPlayer, 0, g.cfg.PlayerCount)
	players := make([]*pb.Player, 0, g.cfg.PlayerCount)
	items := make([]*pb.DroppedEquipmentItem, 0, g.cfg.PlayerCount)
	playersAlive := 0
	for _, player := range g.GameState.Players {
		minGotYou := player.PlayerInfo.Position.X - g.cfg.PlayerRadius
		maxGotYou := player.PlayerInfo.Position.X + g.cfg.PlayerRadius
		intervals := make(map[int]bool)

		for _, sEntity := range g.sortedEntities {
			if sEntity.value > maxGotYou {
				break
			}
			if sEntity.value < minGotYou {
				if sEntity.start {
					intervals[sEntity.entityId] = true
				} else {
					intervals[sEntity.entityId] = false
				}
				continue
			}
			intervals[sEntity.entityId] = true
		}

		playerBody := collision2d.NewCircle(collision2d.NewVector(float64(player.PlayerInfo.Position.X), float64(player.PlayerInfo.Position.Y)),
			float64(g.cfg.PlayerRadius))

		for entityId, in := range intervals {
			if !in {
				continue
			}
			_, info := collision2d.TestPolygonCircle(g.unmovableEntities[entityId], playerBody)
			if info.Overlap < 0 {
				continue
			}
			player.PlayerInfo.Position.X = g.PrevGameStates[g.cfg.GameStatesSaved-1].Players[int(player.PlayerInfo.PlayerId)].Position.X
			player.PlayerInfo.Position.Y = g.PrevGameStates[g.cfg.GameStatesSaved-1].Players[int(player.PlayerInfo.PlayerId)].Position.Y
			break
		}
		if player.PlayerInfo.Hp > 0 {
			playersAlive++
		}
		sortedPlayers = append(sortedPlayers, SortedPlayer{playerId: player.PlayerInfo.PlayerId, value: minGotYou, start: true},
			SortedPlayer{playerId: player.PlayerInfo.PlayerId, value: maxGotYou, start: false})
		players = append(players, player.PlayerInfo.Deepcopy())
	}

	if playersAlive < 2 {
		return true
	}

	sort.SliceStable(sortedPlayers, func(i, j int) bool { return sortedPlayers[i].value < sortedPlayers[j].value })

	for _, item := range g.GameState.Items {
		items = append(items, item.ItemInfo.Deepcopy())
	}

	newPrevGameState := PrevGameState{SortedPlayers: sortedPlayers, Players: players, Items: items}
	g.PrevGameStates = g.PrevGameStates[1:]
	g.PrevGameStates = append(g.PrevGameStates, newPrevGameState)
	return false
}
