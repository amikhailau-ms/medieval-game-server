package connection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
)

type BackoffConfig struct {
	InitialDuration time.Duration
	MaxDuration     time.Duration
	Factor          float64
	Randomization   float64
	MaxInterval     time.Duration
}

type UpdateUserStatsRequest struct {
	AddGames int32 `json:"add_games,omitempty"`
	AddWins  int32 `json:"add_wins,omitempty"`
	AddTop5  int32 `json:"add_top5,omitempty"`
	AddKills int32 `json:"add_kills,omitempty"`
}

type GrantCurrenciesRequest struct {
	AddCoins int `json:"add_coins,omitempty"`
	AddGems  int `json:"add_gems,omitempty"`
}

type UsersServiceConfig struct {
	Enabled                bool
	Address                string
	PutStatsEndpoint       string
	PostCurrenciesEndpoint string
	BaseCoins              int
	DamageCoinsMultiplier  float64
	KillCoinsMultiplier    float64
	Timeout                time.Duration
	Token                  string
	BackoffCfg             *BackoffConfig
}

func (gm *GameManager) SendResults() {
	if gm.cfg.Uscfg.Enabled {
		for _, client := range gm.clients {
			player := gm.gs.GameState.Players[int(client.playerId)]
			err := gm.sendUpdateStatsRequest(client.nickname, player.Position == 1, player.Position <= 5, player.PlayerInfo.Stats.Kills)
			if err != nil {
				log.Printf("Unable to update user \"%v\": %v", client.nickname, err)
			}
			err = gm.sendGrantCurrenciesRequest(client.userId, player.PlayerInfo.Stats.Damage, player.PlayerInfo.Stats.Kills)
			if err != nil {
				log.Printf("Unable to grant currencies to user \"%v\": %v", client.nickname, err)
			}
		}
	}
}

func (gm *GameManager) sendUpdateStatsRequest(nickname string, won, top5 bool, kills int32) error {
	req := UpdateUserStatsRequest{
		AddGames: 1,
		AddKills: kills,
	}
	if won {
		req.AddWins = 1
	}
	if top5 {
		req.AddTop5 = 1
	}
	client := http.Client{
		Timeout: gm.cfg.Uscfg.Timeout,
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return err
	}

	operation := func() error {
		return HttpRequest(&client, http.MethodPut, gm.cfg.Uscfg.Address+gm.cfg.Uscfg.PutStatsEndpoint+nickname, gm.cfg.Uscfg.Token, bytes.NewBuffer(jsonReq))
	}

	err = backoff.Retry(operation, &backoff.ExponentialBackOff{
		InitialInterval:     gm.cfg.Uscfg.BackoffCfg.InitialDuration,
		RandomizationFactor: gm.cfg.Uscfg.BackoffCfg.Randomization,
		Multiplier:          gm.cfg.Uscfg.BackoffCfg.Factor,
		MaxInterval:         gm.cfg.Uscfg.BackoffCfg.MaxDuration,
		MaxElapsedTime:      gm.cfg.Uscfg.BackoffCfg.MaxInterval,
		Clock:               backoff.SystemClock,
	})
	if err != nil {
		return err
	}

	return nil
}

func (gm *GameManager) sendGrantCurrenciesRequest(id string, damage int32, kills int32) error {

	coins := gm.cfg.Uscfg.BaseCoins + int(float64(damage)*gm.cfg.Uscfg.DamageCoinsMultiplier) + int(float64(kills)*gm.cfg.Uscfg.KillCoinsMultiplier)

	req := GrantCurrenciesRequest{
		AddCoins: coins,
		AddGems:  0,
	}
	client := http.Client{
		Timeout: gm.cfg.Uscfg.Timeout,
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return err
	}

	operation := func() error {
		return HttpRequest(&client, http.MethodPost, strings.Replace(gm.cfg.Uscfg.Address+gm.cfg.Uscfg.PostCurrenciesEndpoint, "{id}", id, -1),
			gm.cfg.Uscfg.Token, bytes.NewBuffer(jsonReq))
	}

	err = backoff.Retry(operation, &backoff.ExponentialBackOff{
		InitialInterval:     gm.cfg.Uscfg.BackoffCfg.InitialDuration,
		RandomizationFactor: gm.cfg.Uscfg.BackoffCfg.Randomization,
		Multiplier:          gm.cfg.Uscfg.BackoffCfg.Factor,
		MaxInterval:         gm.cfg.Uscfg.BackoffCfg.MaxDuration,
		MaxElapsedTime:      gm.cfg.Uscfg.BackoffCfg.MaxInterval,
		Clock:               backoff.SystemClock,
	})
	if err != nil {
		return err
	}

	return nil
}

func HttpRequest(client *http.Client, method, url, token string, body io.Reader) error {

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return fmt.Errorf("HttpRequest failed with status: %s", resp.Status)
	}

	return nil
}
