package connection

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"github.com/amikhailau/medieval-game-server/pkg/gamesession"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
)

func TestSendResults(t *testing.T) {

	usersServiceAddress := "users-service:8080"
	usersServicePutEndpoint := "/v1/stats/"
	usersServicePostEndpoint := "/v1/users/{id}/currencies"
	usersServiceToken := "some-admin-token"

	testPlayers := []*gamesession.SyncPlayer{
		{
			Position: 1,
			PlayerInfo: &pb.Player{
				PlayerId: 0,
				Stats: &pb.PlayerStats{
					Kills:  3,
					Damage: 200,
				},
			},
		},
		{
			Position: 2,
			PlayerInfo: &pb.Player{
				PlayerId: 1,
				Stats: &pb.PlayerStats{
					Kills:  2,
					Damage: 150,
				},
			},
		},
		{
			Position: 5,
			PlayerInfo: &pb.Player{
				PlayerId: 2,
				Stats: &pb.PlayerStats{
					Kills:  1,
					Damage: 75,
				},
			},
		},
		{
			Position: 10,
			PlayerInfo: &pb.Player{
				PlayerId: 3,
				Stats: &pb.PlayerStats{
					Kills:  0,
					Damage: 20,
				},
			},
		},
	}
	testClients := map[string]*ClientConnection{
		"0": {
			nickname: "player0",
			userId:   "id-0",
			playerId: 0,
		},
		"1": {
			nickname: "player1",
			userId:   "id-1",
			playerId: 1,
		},
		"2": {
			nickname: "player2",
			userId:   "id-2",
			playerId: 2,
		},
		"3": {
			nickname: "player3",
			userId:   "id-3",
			playerId: 3,
		},
	}
	testGS := &gamesession.GameSession{
		GameState: gamesession.CurrentGameState{
			PlayersLeft: 0,
			Players:     testPlayers,
		},
	}
	testGM := &GameManager{
		clients: testClients,
		cfg: &GameManagerConfig{
			Uscfg: &UsersServiceConfig{
				Enabled:                true,
				Address:                usersServiceAddress,
				PutStatsEndpoint:       usersServicePutEndpoint,
				PostCurrenciesEndpoint: usersServicePostEndpoint,
				Timeout:                5 * time.Second,
				Token:                  usersServiceToken,
				BaseCoins:              50,
				DamageCoinsMultiplier:  1.0,
				KillCoinsMultiplier:    75.0,
				BackoffCfg: &BackoffConfig{
					InitialDuration: 2 * time.Second,
					MaxDuration:     8 * time.Second,
					Randomization:   0.0,
					Factor:          2.0,
					MaxInterval:     20 * time.Second,
				},
			},
		},
		gs: testGS,
	}

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for i := 0; i < 4; i++ {
		player := testPlayers[i]
		client := testClients[strconv.Itoa(i)]
		mockUpdateStats(usersServiceAddress, usersServicePutEndpoint, usersServiceToken, client, player)
		coins := testGM.cfg.Uscfg.BaseCoins + int(float64(player.PlayerInfo.Stats.Damage)*testGM.cfg.Uscfg.DamageCoinsMultiplier) +
			int(float64(player.PlayerInfo.Stats.Kills)*testGM.cfg.Uscfg.KillCoinsMultiplier)
		mockGrantCurrencies(usersServiceAddress, usersServicePostEndpoint, usersServiceToken, client, player, coins)
	}

	testGM.SendResults()

}

func mockUpdateStats(usersServiceAddress, usersServicePutEndpoint, usersServiceToken string, cc *ClientConnection, p *gamesession.SyncPlayer) {
	httpmock.RegisterResponder(http.MethodPut, usersServiceAddress+usersServicePutEndpoint+cc.nickname,
		func(request *http.Request) (*http.Response, error) {
			authToken := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("authorization"), "Bearer "))
			if authToken != usersServiceToken {
				return httpmock.NewStringResponse(401, "HTTP Token: Access denied."), nil
			}

			jsonBody, err := ioutil.ReadAll(request.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, "Request: Malformed body."), nil
			}

			var req UpdateUserStatsRequest
			err = json.Unmarshal(jsonBody, &req)
			if err != nil {
				return httpmock.NewStringResponse(400, "Request: Unable to unmarshal body."), nil
			}

			if req.AddGames != 1 {
				return httpmock.NewStringResponse(400, "Request: Games amount."), nil
			}
			if req.AddKills != p.PlayerInfo.Stats.Kills {
				return httpmock.NewStringResponse(400, "Request: Kills amount."), nil
			}
			if req.AddWins != 1 && p.Position == 1 {
				return httpmock.NewStringResponse(400, "Request: Wins amount."), nil
			}
			if req.AddTop5 != 1 && p.Position <= 5 {
				return httpmock.NewStringResponse(400, "Request: Top5 amount."), nil
			}
			return httpmock.NewJsonResponse(200, nil)
		})
}

func mockGrantCurrencies(usersServiceAddress, usersServicePostEndpoint, usersServiceToken string, cc *ClientConnection, p *gamesession.SyncPlayer, coins int) {
	httpmock.RegisterResponder(http.MethodPost, strings.Replace(usersServiceAddress+usersServicePostEndpoint, "{id}", cc.userId, -1),
		func(request *http.Request) (*http.Response, error) {
			authToken := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("authorization"), "Bearer "))
			if authToken != usersServiceToken {
				return httpmock.NewStringResponse(401, "HTTP Token: Access denied."), nil
			}

			jsonBody, err := ioutil.ReadAll(request.Body)
			if err != nil {
				return httpmock.NewStringResponse(400, "Request: Malformed body."), nil
			}

			var req GrantCurrenciesRequest
			err = json.Unmarshal(jsonBody, &req)
			if err != nil {
				return httpmock.NewStringResponse(400, "Request: Unable to unmarshal body."), nil
			}

			if req.AddCoins != coins {
				return httpmock.NewStringResponse(400, "Request: Coins amount."), nil
			}
			return httpmock.NewJsonResponse(200, nil)
		})
}
