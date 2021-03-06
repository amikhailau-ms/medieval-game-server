package gamesession

import (
	"math"

	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func (g *GameSession) ProcessAction(action *pb.Action, playerId int32) {
	if player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(playerId)]; player.Hp <= 0 {
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
	g.RLock()
	defer g.RUnlock()
	player := g.gameState.players[int(playerId)]
	player.Lock()
	defer player.Unlock()

	//Logic to verify speed?
	player.playerInfo.Position.X += moveAction.Shift.X
	if player.playerInfo.Position.X > g.mapBorderX {
		player.playerInfo.Position.X = g.mapBorderX
	}
	player.playerInfo.Position.Y += moveAction.Shift.Y
	if player.playerInfo.Position.Y > g.mapBorderY {
		player.playerInfo.Position.Y = g.mapBorderY
	}
	player.playerInfo.Angle += moveAction.Angle
	if player.playerInfo.Angle > 2*math.Pi {
		player.playerInfo.Angle -= 2 * math.Pi
	}
	return
}

func (g *GameSession) processAttackAction(attackAction *pb.AttackAction, playerId int32) {
	player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(playerId)]
	weapon := player.Equipment.Weapon
	minGotYou := player.Position.X - weapon.GetWeaponChars().GetRange()
	maxGotYou := player.Position.X + weapon.GetWeaponChars().GetRange()
	intervals := make(map[int32]bool)
	for _, sPlayer := range g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].sortedPlayers {
		if sPlayer.value > maxGotYou {
			break
		}
		if sPlayer.value < minGotYou {
			continue
		}
		intervals[sPlayer.playerId] = true
	}
	for possiblePlayer := range intervals {
		g.processPossibleHit(playerId, possiblePlayer)
	}
}

func (g *GameSession) processPickUpAction(pickUpAction *pb.PickUpAction, playerId int32) {
	player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(playerId)]
	pItemPrev := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].items[int(pickUpAction.ItemId)]
	distance := CalculateDistance(pItemPrev.Position.X, pItemPrev.Position.Y, player.Position.X, player.Position.Y)
	if distance > g.cfg.PlayerPickUpRange {
		return
	}

	var itemToDrop *pb.EquipmentItem

	g.RLock()
	defer g.RUnlock()
	pItem := g.gameState.items[int(pickUpAction.ItemId)]
	pItem.Lock()
	if pItem.pickedUp {
		return
	}
	pItem.pickedUp = true
	pItem.itemInfo.Position.X = -100.0
	pItem.itemInfo.Position.Y = -100.0
	pItem.Unlock()

	playerR := g.gameState.players[int(playerId)]
	playerR.Lock()
	switch pItem.itemInfo.Item.Type {
	case pb.EquipmentItemType_ARMOR:
		itemToDrop = playerR.playerInfo.Equipment.Armor
		playerR.playerInfo.Equipment.Armor = pItem.itemInfo.Item
	case pb.EquipmentItemType_HELMET:
		itemToDrop = playerR.playerInfo.Equipment.Helmet
		playerR.playerInfo.Equipment.Helmet = pItem.itemInfo.Item
		playerR.playerInfo.Hp += playerR.playerInfo.Equipment.Helmet.GetHpBuff()
	case pb.EquipmentItemType_WEAPON:
		itemToDrop = playerR.playerInfo.Equipment.Weapon
		playerR.playerInfo.Equipment.Weapon = pItem.itemInfo.Item
	}
	playerR.Unlock()

	if itemToDrop.Rarity != pb.EquipmentItemRarity_DEFAULT {
		g.dropItem(playerId, itemToDrop.ItemId)
	}

	return
}

func (g *GameSession) processDropAction(dropAction *pb.DropAction, playerId int32) {

	player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(playerId)]

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

	g.RLock()
	defer g.RUnlock()

	g.dropItem(playerId, itemToDrop.ItemId)

	return
}

func (g *GameSession) dropItem(playerId, itemId int32) {
	player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(playerId)]

	newPosX := player.Position.X + float32(g.cfg.PlayerDropRange*math.Cos(float64(player.Angle)))
	newPosY := player.Position.Y + float32(g.cfg.PlayerDropRange*math.Sin(float64(player.Angle)))

	pItem := g.gameState.items[int(itemId)]
	pItem.Lock()
	pItem.itemInfo.Position.X = newPosX
	pItem.itemInfo.Position.Y = newPosY
	pItem.pickedUp = false
	pItem.Unlock()

	playerR := g.gameState.players[int(playerId)]
	playerR.Lock()
	defer playerR.Unlock()

	switch pItem.itemInfo.Item.Type {
	case pb.EquipmentItemType_ARMOR:
		playerR.playerInfo.Equipment.Armor = nil
	case pb.EquipmentItemType_HELMET:
		if playerR.playerInfo.Equipment.Helmet != nil {
			playerR.playerInfo.Hp -= playerR.playerInfo.Equipment.Helmet.GetHpBuff()
			if playerR.playerInfo.Hp <= 0 {
				playerR.playerInfo.Hp = 1
			}
			playerR.playerInfo.Equipment.Helmet = nil
		}
	case pb.EquipmentItemType_WEAPON:
		playerR.playerInfo.Equipment.Weapon = g.cfg.DefaultWeapon.Deepcopy()
	default:
		return
	}
}

func (g *GameSession) processPossibleHit(attPlayerId, defPlayerId int32) {
	player := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[int(attPlayerId)]
	weapon := player.Equipment.Weapon
	angle := player.Angle
	pPlayer := g.prevGameStates[g.cfg.GameStatesSaved-g.cfg.GameStatesShiftBack].players[defPlayerId]
	distance := CalculateDistance(player.Position.X, player.Position.Y, pPlayer.Position.X, pPlayer.Position.Y)
	if distance > 2*g.cfg.PlayerRadius {
		return
	}

	angleBetween := float32(math.Atan(float64((pPlayer.Position.Y - player.Position.Y) / (pPlayer.Position.X - player.Position.X))))
	angleCone := float32(math.Atan(float64(g.cfg.PlayerRadius / distance)))
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

	if (minAngle < maxAngle && ((minAngle < minAngleHits && minAngleHits < maxAngle) || (minAngle < maxAngleHits && maxAngleHits < maxAngle))) ||
		(minAngle > maxAngle && ((minAngle < minAngleHits || minAngleHits < maxAngle) || (minAngle < maxAngleHits || maxAngleHits < maxAngle))) {
		knockbackY := weapon.GetWeaponChars().KnockbackPower * float32(math.Cos(float64(angleBetween)))
		knockbackX := weapon.GetWeaponChars().KnockbackPower * float32(math.Sin(float64(angleBetween)))
		attackValue := weapon.GetWeaponChars().AttackPower
		if pPlayer.Equipment.Armor != nil {
			attackValue -= pPlayer.Equipment.Armor.GetDamageReduction()
		}
		g.RLock()
		playerToUpdate := g.gameState.players[int(defPlayerId)]
		playerToUpdate.Lock()
		playerToUpdate.playerInfo.Hp -= attackValue
		playerToUpdate.playerInfo.Position.X += knockbackX
		playerToUpdate.playerInfo.Position.Y += knockbackY
		playerToUpdate.Unlock()
		g.RUnlock()
	}
}
