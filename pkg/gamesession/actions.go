package gamesession

import (
	"math"

	"github.com/Tarliton/collision2d"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func (g *GameSession) ProcessAction(action *pb.Action, playerId int32) {
	g.RLock()
	defer g.RUnlock()
	if player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(playerId)]; player.Hp <= 0 {
		return
	}

	if moveAction := action.GetMove(); moveAction != nil {
		g.processMoveAction(moveAction, playerId)
		return
	}

	if attackAction := action.GetAttack(); attackAction != nil {
		g.processAttackAction(attackAction, playerId)
		return
	}

	if pickUpAction := action.GetPickUp(); pickUpAction != nil {
		g.processPickUpAction(pickUpAction, playerId)
		return
	}

	if dropAction := action.GetDrop(); dropAction != nil {
		g.processDropAction(dropAction, playerId)
		return
	}

}

func (g *GameSession) processMoveAction(moveAction *pb.MovementAction, playerId int32) {
	player := g.GameState.Players[int(playerId)]
	minGotYou := player.PlayerInfo.Position.X - g.cfg.PlayerRadius
	maxGotYou := player.PlayerInfo.Position.X + g.cfg.PlayerRadius
	player.Lock()
	//Logic to verify speed?
	if moveAction.Shift != nil {
		player.PlayerInfo.Position.X += moveAction.Shift.X
		if player.PlayerInfo.Position.X > g.mapBorderX {
			player.PlayerInfo.Position.X = g.mapBorderX
		}
		player.PlayerInfo.Position.Y += moveAction.Shift.Y
		if player.PlayerInfo.Position.Y > g.mapBorderY {
			player.PlayerInfo.Position.Y = g.mapBorderY
		}
	}
	player.PlayerInfo.Angle += moveAction.Angle
	if player.PlayerInfo.Angle > 2*math.Pi {
		player.PlayerInfo.Angle -= 2 * math.Pi
	}
	player.Unlock()

	if moveAction.Shift != nil {
		playerBody := collision2d.NewPolygon(collision2d.NewVector(0, 0), collision2d.NewVector(0, 0), 0, []float64{
			float64(maxGotYou), float64(player.PlayerInfo.Position.Y - moveAction.Shift.Y),
			float64(minGotYou), float64(player.PlayerInfo.Position.Y - moveAction.Shift.Y),
			float64(minGotYou + moveAction.Shift.X), float64(player.PlayerInfo.Position.Y),
			float64(maxGotYou + moveAction.Shift.X), float64(player.PlayerInfo.Position.Y),
		})

		intervals := make(map[int]bool)

		for _, sEntity := range g.sortedEntities {
			if sEntity.value > maxGotYou+moveAction.Shift.X {
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

		for entityId, in := range intervals {
			if !in {
				continue
			}
			_, info := collision2d.TestPolygonPolygon(playerBody, g.unmovableEntities[entityId])
			if info.Overlap < 0 {
				continue
			}
			player.Lock()
			player.PlayerInfo.Position.X = g.PrevGameStates[g.cfg.GameStatesSaved-1].Players[int(player.PlayerInfo.PlayerId)].Position.X
			player.PlayerInfo.Position.Y = g.PrevGameStates[g.cfg.GameStatesSaved-1].Players[int(player.PlayerInfo.PlayerId)].Position.Y
			player.Unlock()
			break
		}
	}
	return
}

func (g *GameSession) processAttackAction(attackAction *pb.AttackAction, playerId int32) {
	player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(playerId)]
	weapon := player.Equipment.Weapon
	minGotYou := player.Position.X - weapon.GetWeaponChars().GetRange()
	maxGotYou := player.Position.X + weapon.GetWeaponChars().GetRange()
	intervals := make(map[int32]bool)
	for _, sPlayer := range g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].SortedPlayers {
		if sPlayer.playerId == playerId {
			continue
		}
		if sPlayer.value > maxGotYou {
			break
		}
		if sPlayer.value < minGotYou {
			continue
		}
		intervals[sPlayer.playerId] = true
	}
	g.AttackNotifications <- playerId
	for possiblePlayer := range intervals {
		g.processPossibleHit(playerId, possiblePlayer)
	}
}

func (g *GameSession) processPickUpAction(pickUpAction *pb.PickUpAction, playerId int32) {

	player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(playerId)]
	pItemPrev := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Items[int(pickUpAction.ItemId)]
	distance := CalculateDistance(pItemPrev.Position.X, pItemPrev.Position.Y, player.Position.X, player.Position.Y)
	if distance > g.cfg.PlayerPickUpRange {
		return
	}

	var itemToDrop *pb.EquipmentItem

	pItem := g.GameState.Items[int(pickUpAction.ItemId)]
	pItem.Lock()
	if pItem.pickedUp {
		return
	}
	pItem.pickedUp = true
	pItem.ItemInfo.Position.X = -100.0
	pItem.ItemInfo.Position.Y = -100.0
	pItem.Unlock()

	playerR := g.GameState.Players[int(playerId)]
	playerR.Lock()
	switch pItem.ItemInfo.Item.Type {
	case pb.EquipmentItemType_ARMOR:
		itemToDrop = playerR.PlayerInfo.Equipment.Armor
		playerR.PlayerInfo.Equipment.Armor = pItem.ItemInfo.Item
	case pb.EquipmentItemType_HELMET:
		itemToDrop = playerR.PlayerInfo.Equipment.Helmet
		playerR.PlayerInfo.Equipment.Helmet = pItem.ItemInfo.Item
		playerR.PlayerInfo.Hp += pItem.ItemInfo.Item.GetHpBuff()
	case pb.EquipmentItemType_WEAPON:
		itemToDrop = playerR.PlayerInfo.Equipment.Weapon
		playerR.PlayerInfo.Equipment.Weapon = pItem.ItemInfo.Item
	}
	playerR.Unlock()

	if itemToDrop != nil && itemToDrop.Rarity != pb.EquipmentItemRarity_DEFAULT {
		g.dropItem(playerId, itemToDrop.ItemId, false)
	}

	return
}

func (g *GameSession) processDropAction(dropAction *pb.DropAction, playerId int32) {

	player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(playerId)]

	var itemToDrop *pb.EquipmentItem
	switch dropAction.Slot {
	case pb.EquipmentItemType_ARMOR:
		itemToDrop = player.Equipment.Armor
	case pb.EquipmentItemType_HELMET:
		itemToDrop = player.Equipment.Helmet
	case pb.EquipmentItemType_WEAPON:
		if player.Equipment.Weapon.Rarity == pb.EquipmentItemRarity_DEFAULT {
			return
		}
		itemToDrop = player.Equipment.Weapon
	default:
		return
	}

	if itemToDrop == nil {
		return
	}

	g.dropItem(playerId, itemToDrop.ItemId, true)

	return
}

func (g *GameSession) dropItem(playerId, itemId int32, needToReset bool) {
	player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(playerId)]

	newPosX := player.Position.X + g.cfg.PlayerDropRange*float32(math.Cos(float64(player.Angle)))
	newPosY := player.Position.Y + g.cfg.PlayerDropRange*float32(math.Sin(float64(player.Angle)))

	pItem := g.GameState.Items[int(itemId)]
	pItem.Lock()
	pItem.ItemInfo.Position.X = newPosX
	pItem.ItemInfo.Position.Y = newPosY
	pItem.pickedUp = false
	pItem.Unlock()

	playerR := g.GameState.Players[int(playerId)]
	playerR.Lock()
	defer playerR.Unlock()

	switch pItem.ItemInfo.Item.Type {
	case pb.EquipmentItemType_ARMOR:
		if needToReset {
			playerR.PlayerInfo.Equipment.Armor = nil
		}
	case pb.EquipmentItemType_HELMET:
		if playerR.PlayerInfo.Equipment.Helmet != nil {
			playerR.PlayerInfo.Hp -= pItem.ItemInfo.Item.GetHpBuff()
			if playerR.PlayerInfo.Hp <= 0 {
				playerR.PlayerInfo.Hp = 1
			}
			if needToReset {
				playerR.PlayerInfo.Equipment.Helmet = nil
			}
		}
	case pb.EquipmentItemType_WEAPON:
		if needToReset {
			playerR.PlayerInfo.Equipment.Weapon = g.cfg.DefaultWeapon.Deepcopy()
		}
	default:
		return
	}
}

func (g *GameSession) processPossibleHit(attPlayerId, defPlayerId int32) {
	player := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[int(attPlayerId)]
	weapon := player.Equipment.Weapon
	angle := player.Angle
	pPlayer := g.PrevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].Players[defPlayerId]
	distance := CalculateDistance(player.Position.X, player.Position.Y, pPlayer.Position.X, pPlayer.Position.Y)
	if distance > g.cfg.PlayerRadius+weapon.GetWeaponChars().Range {
		return
	}

	angleBetween := float32(math.Atan(float64((pPlayer.Position.Y - player.Position.Y) / (pPlayer.Position.X - player.Position.X))))
	angleCone := float32(math.Asin(float64(g.cfg.PlayerRadius / distance)))
	minAngleHits := angleBetween - angleCone
	if minAngleHits < 0 {
		minAngleHits += 2 * math.Pi
	}
	maxAngleHits := angleBetween + angleCone
	if maxAngleHits > 2*math.Pi {
		maxAngleHits -= 2 * math.Pi
	}

	minAngle := angle - weapon.GetWeaponChars().AttackCone
	if minAngle < 0 {
		minAngle += 2 * math.Pi
	}
	maxAngle := angle + weapon.GetWeaponChars().AttackCone
	if maxAngle > 2*math.Pi {
		maxAngle -= 2 * math.Pi
	}

	if SectorCollision(minAngleHits, maxAngleHits, minAngle, maxAngle) {
		knockbackY := weapon.GetWeaponChars().KnockbackPower * float32(math.Sin(float64(angleBetween)))
		knockbackX := weapon.GetWeaponChars().KnockbackPower * float32(math.Cos(float64(angleBetween)))
		attackValue := weapon.GetWeaponChars().AttackPower
		if pPlayer.Equipment.Armor != nil {
			attackValue -= pPlayer.Equipment.Armor.GetDamageReduction()
		}
		playerToUpdate := g.GameState.Players[int(defPlayerId)]
		playerToUpdate.Lock()
		playerToUpdate.PlayerInfo.Hp -= attackValue
		hpLeft := playerToUpdate.PlayerInfo.Hp
		playerToUpdate.PlayerInfo.Position.X += knockbackX
		playerToUpdate.PlayerInfo.Position.Y += knockbackY
		playerToUpdate.Unlock()
		playerCurr := g.GameState.Players[int(attPlayerId)]
		playerCurr.PlayerInfo.Stats.Damage += attackValue
		if hpLeft <= 0 {
			g.KillNotifications <- KillInfo{
				Actor:    player.Nickname,
				Receiver: pPlayer.Nickname,
			}
			g.deadPlayers <- pPlayer.PlayerId
			playerCurr.PlayerInfo.Stats.Kills += 1
		}
	}
}
