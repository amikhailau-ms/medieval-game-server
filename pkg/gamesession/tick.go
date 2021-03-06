package gamesession

import (
	"sort"

	"github.com/Tarliton/collision2d"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func (g *GameSession) DoSessionTick() {
	g.Lock()
	defer g.Unlock()

	sortedPlayers := make([]SortedPlayer, 0, g.cfg.PlayerCount)
	players := make([]*pb.Player, 0, g.cfg.PlayerCount)
	items := make([]*pb.DroppedEquipmentItem, 0, g.cfg.PlayerCount)
	for _, player := range g.gameState.players {
		minGotYou := player.playerInfo.Position.X - g.cfg.PlayerRadius
		maxGotYou := player.playerInfo.Position.X + g.cfg.PlayerRadius
		intervals := make(map[int]bool)

		for _, sEntity := range g.sortedEntities {
			if sEntity.value > maxGotYou {
				break
			}
			if sEntity.value < minGotYou {
				continue
			}
			intervals[sEntity.entityId] = true
		}

		playerBody := collision2d.NewCircle(collision2d.NewVector(float64(player.playerInfo.Position.X), float64(player.playerInfo.Position.Y)),
			float64(g.cfg.PlayerRadius))

		for entityId := range intervals {
			areColliding, _ := collision2d.TestCirclePolygon(playerBody, g.unmovableEntities[entityId])
			if !areColliding {
				continue
			}
			player.playerInfo.Position.X = g.prevGameStates[g.cfg.GameStatesSaved-1].players[int(player.playerInfo.PlayerId)].Position.X
			player.playerInfo.Position.Y = g.prevGameStates[g.cfg.GameStatesSaved-1].players[int(player.playerInfo.PlayerId)].Position.Y
			break
		}
		sortedPlayers = append(sortedPlayers, SortedPlayer{playerId: player.playerInfo.PlayerId, value: minGotYou, start: true},
			SortedPlayer{playerId: player.playerInfo.PlayerId, value: maxGotYou, start: false})
		players = append(players, player.playerInfo.Deepcopy())
	}
	sort.SliceStable(sortedPlayers, func(i, j int) bool { return sortedPlayers[i].value < sortedPlayers[j].value })

	for _, item := range g.gameState.items {
		items = append(items, item.itemInfo.Deepcopy())
	}

	newPrevGameState := PrevGameState{sortedPlayers: sortedPlayers, players: players, items: items}
	g.prevGameStates = g.prevGameStates[1:]
	g.prevGameStates = append(g.prevGameStates, newPrevGameState)
	return
}
