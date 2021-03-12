package pb

func (x *WeaponCharacteristics) Deepcopy() *WeaponCharacteristics {
	weaponCharacteristics := WeaponCharacteristics{
		AttackPower:    x.AttackPower,
		Range:          x.Range,
		AttackCone:     x.AttackCone,
		KnockbackPower: x.KnockbackPower,
	}
	return &weaponCharacteristics
}

func (x *EquipmentItem) Deepcopy() *EquipmentItem {
	equipmentItem := EquipmentItem{
		Type:   x.Type,
		Rarity: x.Rarity,
		ItemId: x.ItemId,
	}
	if chars := x.GetWeaponChars(); chars != nil {
		newChars := chars.Deepcopy()
		equipmentItem.Characteristics = &EquipmentItem_WeaponChars{WeaponChars: newChars}
	}
	if hpBuff := x.GetHpBuff(); hpBuff != 0 {
		equipmentItem.Characteristics = &EquipmentItem_HpBuff{HpBuff: hpBuff}
	}
	if damageReduction := x.GetDamageReduction(); damageReduction != 0 {
		equipmentItem.Characteristics = &EquipmentItem_DamageReduction{DamageReduction: damageReduction}
	}
	return &equipmentItem
}

func (x *Vector) Deepcopy() *Vector {
	vector := Vector{
		X: x.X,
		Y: x.Y,
	}
	return &vector
}

func (x *DroppedEquipmentItem) Deepcopy() *DroppedEquipmentItem {
	var newEquipmentInfo *EquipmentItem
	var newPosition *Vector
	if x.Item != nil {
		newEquipmentInfo = x.Item.Deepcopy()
	}
	if x.Position != nil {
		newPosition = x.Position.Deepcopy()
	}
	droppedEquipmentItem := DroppedEquipmentItem{
		Item:     newEquipmentInfo,
		Position: newPosition,
	}
	return &droppedEquipmentItem
}

func (x *PlayerEquipment) Deepcopy() *PlayerEquipment {
	var newHelmet *EquipmentItem
	var newArmor *EquipmentItem
	var newWeapon *EquipmentItem
	if x.Helmet != nil {
		newHelmet = x.Helmet.Deepcopy()
	}
	if x.Armor != nil {
		newArmor = x.Armor.Deepcopy()
	}
	if x.Weapon != nil {
		newWeapon = x.Weapon.Deepcopy()
	}
	playerEquipment := PlayerEquipment{
		Helmet: newHelmet,
		Armor:  newArmor,
		Weapon: newWeapon,
	}
	return &playerEquipment
}

func (x *Player) Deepcopy() *Player {
	var newEquipment *PlayerEquipment
	if x.Equipment != nil {
		newEquipment = x.Equipment.Deepcopy()
	}
	var newPosition *Vector
	if x.Position != nil {
		newPosition = x.Position.Deepcopy()
	}
	player := Player{
		Nickname:  x.Nickname,
		Hp:        x.Hp,
		Equipment: newEquipment,
		UserId:    x.UserId,
		Position:  newPosition,
		Angle:     x.Angle,
		PlayerId:  x.PlayerId,
	}
	return &player
}
