package gamesession

import (
	"math"
	"testing"

	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func TestProcessMoveAction(t *testing.T) {
	gs, err := MakeTestGameSession()
	if err != nil {
		t.Fatalf("unable to create test game session: %v", err)
	}

	testCases := []struct {
		name            string
		moveAction      *pb.MovementAction
		playerId        int32
		expectedPlayerX float32
		expectedPlayerY float32
		expectedAngle   float32
	}{
		{
			name: "valid movement",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 10, Y: 20},
			},
			playerId:        0,
			expectedPlayerX: 60,
			expectedPlayerY: 60,
			expectedAngle:   math.Pi / 2,
		},
		{
			name: "invalid movement",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 35, Y: -10},
			},
			playerId:        0,
			expectedPlayerX: 50,
			expectedPlayerY: 40,
			expectedAngle:   math.Pi / 2,
		},
		{
			name: "valid movement with turn",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 10, Y: 10},
				Angle: 0.785,
			},
			playerId:        0,
			expectedPlayerX: 60,
			expectedPlayerY: 50,
			expectedAngle:   math.Pi/2 + 0.785,
		},
		{
			name: "movement out of bounds Y",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 0, Y: 55},
			},
			playerId:        0,
			expectedPlayerX: 60,
			expectedPlayerY: 100,
			expectedAngle:   math.Pi/2 + 0.785,
		},
		{
			name: "movement out of bounds X",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 45, Y: 0},
			},
			playerId:        0,
			expectedPlayerX: 100,
			expectedPlayerY: 100,
			expectedAngle:   math.Pi/2 + 0.785,
		},
		{
			name: "complete turn around",
			moveAction: &pb.MovementAction{
				Shift: &pb.Vector{X: 0, Y: 0},
				Angle: 5.672,
			},
			playerId:        0,
			expectedPlayerX: 100,
			expectedPlayerY: 100,
			expectedAngle:   5.672 + 0.785 - 3*math.Pi/2,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			gs.processMoveAction(test.moveAction, test.playerId)
			player := gs.GameState.Players[test.playerId]
			if player.PlayerInfo.Position.X != test.expectedPlayerX {
				t.Fatalf("expected X coordinate of player #%v: %.3f, got: %.3f", test.playerId, test.expectedPlayerX, player.PlayerInfo.Position.X)
			}
			if player.PlayerInfo.Position.Y != test.expectedPlayerY {
				t.Fatalf("expected Y coordinate of player #%v: %.3f, got: %.3f", test.playerId, test.expectedPlayerY, player.PlayerInfo.Position.Y)
			}
			if player.PlayerInfo.Angle-test.expectedAngle > 0.01 {
				t.Fatalf("expected angle of player #%v: %.3f, got: %.3f", test.playerId, test.expectedAngle, player.PlayerInfo.Angle)
			}
		})
	}
}

func TestProcessPlayerScenario(t *testing.T) {
	gs, err := MakeTestGameSession()
	if err != nil {
		t.Fatalf("unable to create test game session: %v", err)
	}

	actions := []*pb.Action{
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: 20, Y: -20},
			}},
		},
		{
			Action: &pb.Action_Attack{Attack: &pb.AttackAction{}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Angle: -math.Pi / 2,
			}},
		},
		{
			Action: &pb.Action_Attack{Attack: &pb.AttackAction{}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: -20, Y: 0},
			}},
		},
		{
			Action: &pb.Action_PickUp{PickUp: &pb.PickUpAction{
				ItemId: 4,
			}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: -10, Y: 0},
			}},
		},
		{
			Action: &pb.Action_PickUp{PickUp: &pb.PickUpAction{
				ItemId: 4,
			}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: 25, Y: 60},
			}},
		},
		{
			Action: &pb.Action_Attack{Attack: &pb.AttackAction{}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: 0, Y: -10},
				Angle: 1.31,
			}},
		},
		{
			Action: &pb.Action_Attack{Attack: &pb.AttackAction{}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: -20, Y: 10},
			}},
		},
		{
			Action: &pb.Action_PickUp{PickUp: &pb.PickUpAction{
				ItemId: 0,
			}},
		},
		{
			Action: &pb.Action_Move{Move: &pb.MovementAction{
				Shift: &pb.Vector{X: -10, Y: 5},
				Angle: -1.31,
			}},
		},
		{
			Action: &pb.Action_PickUp{PickUp: &pb.PickUpAction{
				ItemId: 5,
			}},
		},
	}

	gs.ProcessAction(actions[0], 0)
	gs.DoSessionTick()
	t.Log("Action #0.")

	gs.ProcessAction(actions[1], 0)
	if gs.GameState.Players[1].PlayerInfo.Hp < 100 {
		t.Fatal("Hit on player #1 registered when it should not be - action #1")
	}
	checkIfAttackNotificationIsPresent(t, gs, 1)
	t.Log("Action #1.")

	gs.ProcessAction(actions[2], 0)
	gs.DoSessionTick()
	t.Log("Action #2.")

	gs.ProcessAction(actions[3], 0)
	if gs.GameState.Players[1].PlayerInfo.Hp != 90 {
		t.Fatal("Hit on player #1 not registered properly - action #3")
	}
	if gs.GameState.Players[0].PlayerInfo.Stats.Damage != 10 {
		t.Fatal("Damage stats for player #0 not updated properly - action #3")
	}
	checkIfAttackNotificationIsPresent(t, gs, 3)
	t.Log("Action #3.")

	gs.ProcessAction(actions[4], 0)
	gs.DoSessionTick()
	t.Log("Action #4.")

	gs.ProcessAction(actions[5], 0)
	if gs.GameState.Players[0].PlayerInfo.Equipment.Weapon.Rarity != pb.EquipmentItemRarity_DEFAULT {
		t.Fatal("Item picked up when it should not be (player side) - action #5")
	}
	if gs.GameState.Items[4].pickedUp {
		t.Fatal("Item picked up when it should not be (item side)- action #5")
	}
	t.Log("Action #5.")

	gs.ProcessAction(actions[6], 0)
	gs.DoSessionTick()
	t.Log("Action #6.")

	gs.ProcessAction(actions[7], 0)
	if gs.GameState.Players[0].PlayerInfo.Equipment.Weapon.Rarity != pb.EquipmentItemRarity_COMMON {
		t.Fatal("Item not picked up when it should be (player side) - action #7")
	}
	if !gs.GameState.Items[4].pickedUp {
		t.Fatal("Item picked up when it should not be (item side)- action #7")
	}
	t.Log("Action #7.")

	gs.ProcessAction(actions[8], 0)
	gs.DoSessionTick()
	t.Log("Action #8.")

	gs.ProcessAction(actions[9], 0)
	if gs.GameState.Players[2].PlayerInfo.Hp != 60 {
		t.Fatal("Hit on player #2 not registered properly - action #9")
	}
	if gs.GameState.Players[3].PlayerInfo.Hp != 65 {
		t.Fatal("Hit on player #3 not registered properly - action #9")
	}
	checkIfAttackNotificationIsPresent(t, gs, 9)
	if gs.GameState.Players[0].PlayerInfo.Stats.Damage != 35 {
		t.Fatal("Damage stats for player #0 not updated properly - action #9")
	}
	t.Log("Action #9.")

	gs.ProcessAction(actions[10], 0)
	gs.DoSessionTick()
	t.Log("Action #10.")

	gs.ProcessAction(actions[11], 0)
	if gs.GameState.Players[2].PlayerInfo.Hp != 40 {
		t.Fatal("Hit on player #2 not registered properly - action #11")
	}
	if gs.GameState.Players[3].PlayerInfo.Hp != 65 {
		t.Fatal("Hit on player #3 registered when it should not be - action #11")
	}
	t.Log("Action #11.")

	for gs.GameState.Players[2].PlayerInfo.Hp > 0 {
		gs.ProcessAction(actions[11], 0)
		checkIfAttackNotificationIsPresent(t, gs, 11)
	}
	if gs.GameState.Players[0].PlayerInfo.Stats.Damage != 95 {
		t.Fatal("Damage stat for player #0 not updated properly - action #11 cycle")
	}
	gs.DoSessionTick()
	if gs.GameState.Players[0].PlayerInfo.Stats.Kills != 1 {
		t.Fatal("Kill stat for player #0 not updated properly - action #11 cycle")
	}
	if gs.GameState.Players[2].Position != 4 {
		t.Fatal("Position stat for player #2 not updated properly - action #11 cycle")
	}
	if gs.GameState.PlayersLeft != 3 {
		t.Fatal("Players left stat not updated properly - action #11 cycle")
	}
	checkIfKillNotificationIsPresent(t, gs, 11)
	t.Log("Action #11 cycle.")

	gs.ProcessAction(actions[12], 0)
	gs.DoSessionTick()
	t.Log("Action #12.")

	gs.ProcessAction(actions[13], 0)
	if gs.GameState.Players[0].PlayerInfo.Equipment.Helmet == nil {
		t.Fatal("Item not picked up when it should be (player side) - action #13")
	}
	if gs.GameState.Players[0].PlayerInfo.Hp != 120 {
		t.Fatal("Item hp buff not reflected on player - action #13")
	}
	if !gs.GameState.Items[0].pickedUp {
		t.Fatal("Item picked up when it should not be (item side)- action #13")
	}
	t.Log("Action #13.")

	gs.ProcessAction(actions[14], 0)
	gs.DoSessionTick()
	t.Log("Action #14.")

	gs.ProcessAction(actions[15], 0)
	if gs.GameState.Players[0].PlayerInfo.Equipment.Helmet.Rarity != pb.EquipmentItemRarity_RARE {
		t.Fatal("Item not picked up when it should be (player side) - action #15")
	}
	if gs.GameState.Players[0].PlayerInfo.Hp != 130 {
		t.Fatal("Item hp buff not reflected on player - action #15")
	}
	if !gs.GameState.Items[5].pickedUp {
		t.Fatal("Item picked up when it should not be (item side)- action #15")
	}
	if gs.GameState.Items[0].pickedUp {
		t.Fatal("Item not dropped when it should be (item side)- action #15")
	}
	if gs.GameState.Items[0].ItemInfo.Position.X != 47 || gs.GameState.Items[0].ItemInfo.Position.Y != 85 {
		t.Fatal("Item not dropped where it should be (item side)- action #15")
	}
	t.Log("Action #15.")
}

func checkIfAttackNotificationIsPresent(t *testing.T, gs *GameSession, actionId int) {

	select {
	case value := <-gs.AttackNotifications:
		if value != 0 {
			t.Fatalf("Wrong player id in notification - expected #0, got #%d - action #%d", value, actionId)
		}
	default:
		t.Fatalf("Expected attack notification to be created - action #%v", actionId)
	}

}

func checkIfKillNotificationIsPresent(t *testing.T, gs *GameSession, actionId int) {

	select {
	case value := <-gs.KillNotifications:
		if value.Actor != "player" {
			t.Fatalf("Wrong actor nickname in kill notification - expected player, got #%v - action #%v", value.Actor, actionId)
		}
		if value.Receiver != "enemy2" {
			t.Fatalf("Wrong actor nickname in kill notification - expected enemy2, got #%v - action #%v", value.Receiver, actionId)
		}
	default:
		t.Fatalf("Expected kill notification to be created - action #%v", actionId)
	}

}
