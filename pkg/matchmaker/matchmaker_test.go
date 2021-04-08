package matchmaker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amikhailau/medieval-game-server/pkg/auth"
	"github.com/amikhailau/medieval-game-server/pkg/mpb"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
)

func init() {
	ids := []string{"1", "2", "3", "4", "5"}

	for _, id := range ids {
		claims := &auth.GameClaims{
			UserId:    id,
			UserName:  "name" + id,
			UserEmail: id + "@email.com",
			StandardClaims: jwt.StandardClaims{
				Audience:  "medieval",
				ExpiresAt: time.Now().Add(8 * time.Hour).Unix(),
				Id:        uuid.New().String(),
				IssuedAt:  time.Now().Unix(),
				Issuer:    "users-service",
				NotBefore: time.Now().Unix(),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
		tokenString, _ := token.SignedString([]byte("somehmackey"))

		jwts = append(jwts, tokenString)
		userKeys = append(userKeys, UserPrefix+id)
	}
}

var (
	jwts     = []string{}
	userKeys = []string{}
)

func createTestMatchmakeServer() *MatchmakerServer {
	cache := cache.New(2*time.Minute, 5*time.Minute)

	return &MatchmakerServer{
		playersInQueue:    make([]*PlayerData, 0),
		playersMatchmaked: make(map[string]bool),
		matchData:         cache,
		cfg: &MatchmakerServerConfig{
			LobbySize:        2,
			MatchmakingDelay: 2 * time.Second,
			AgonesClient:     nil,
			MatchKeep:        10 * time.Minute,
		},
	}
}

func TestMatchmake(t *testing.T) {
	ms := createTestMatchmakeServer()
	ctxOr := ctxlogrus.ToContext(context.Background(), logrus.NewEntry(logrus.New()))

	ms.playersMatchmaked["1"] = true
	ms.playersMatchmaked["2"] = true
	ms.playersMatchmaked["5"] = true

	ms.playersInQueue = append(ms.playersInQueue, &PlayerData{UserId: "5"}, &PlayerData{UserId: "1"}, &PlayerData{UserId: "2"})

	testCases := []struct {
		name             string
		userId           int
		pqChangeExpected bool
	}{
		{
			name:             "add new player to queue",
			userId:           3,
			pqChangeExpected: true,
		},
		{
			name:             "add existing player to queue",
			userId:           1,
			pqChangeExpected: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			userId := strconv.Itoa(test.userId)
			jwt := jwts[test.userId-1]
			ctx := metadata.NewIncomingContext(ctxOr, metadata.Pairs("authorization", "Bearer "+jwt))
			prevLen := len(ms.playersInQueue)

			_, err := ms.Matchmake(ctx, &mpb.MatchmakeRequest{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if test.pqChangeExpected {
				gotUserId := ms.playersInQueue[len(ms.playersInQueue)-1].UserId
				if gotUserId != userId {
					t.Errorf("expected last in queue: %v, got: %v", userId, gotUserId)
				}
			} else {
				if prevLen != len(ms.playersInQueue) {
					t.Errorf("expected length of queue: %v, got: %v", prevLen, len(ms.playersInQueue))
				}
			}
		})
	}
}

func TestCancelMatchmake(t *testing.T) {
	ms := createTestMatchmakeServer()
	ctxOr := ctxlogrus.ToContext(context.Background(), logrus.NewEntry(logrus.New()))

	ms.playersMatchmaked["1"] = true
	ms.playersMatchmaked["2"] = true
	ms.playersMatchmaked["5"] = true

	ms.playersInQueue = append(ms.playersInQueue, &PlayerData{UserId: "5"}, &PlayerData{UserId: "1"}, &PlayerData{UserId: "2"})

	testCases := []struct {
		name             string
		userId           int
		pqChangeExpected bool
	}{
		{
			name:             "remove existing player from queue",
			userId:           1,
			pqChangeExpected: true,
		},
		{
			name:             "remove nonexistent player from queue",
			userId:           3,
			pqChangeExpected: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			userId := strconv.Itoa(test.userId)
			jwt := jwts[test.userId-1]
			ctx := metadata.NewIncomingContext(ctxOr, metadata.Pairs("authorization", "Bearer "+jwt))
			prevLen := len(ms.playersInQueue)

			_, err := ms.CancelMatchmake(ctx, &mpb.CancelMatchmakeRequest{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if test.pqChangeExpected {
				if prevLen-1 != len(ms.playersInQueue) {
					t.Errorf("expected length of queue: %v, got: %v", prevLen-1, len(ms.playersInQueue))
				}
				for _, player := range ms.playersInQueue {
					if player.UserId == userId {
						t.Errorf("expected user #%v to be removed from queue", userId)
					}
				}
			} else {
				if prevLen != len(ms.playersInQueue) {
					t.Errorf("expected length of queue: %v, got: %v", prevLen, len(ms.playersInQueue))
				}
			}
		})
	}
}

func TestCheckMatchmakeStatus(t *testing.T) {
	ms := createTestMatchmakeServer()
	ctxOr := ctxlogrus.ToContext(context.Background(), logrus.NewEntry(logrus.New()))

	ms.playersMatchmaked["1"] = true
	ms.playersMatchmaked["2"] = true
	ms.playersMatchmaked["3"] = true
	ms.playersMatchmaked["4"] = true

	ms.matchData.Set(UserPrefix+"1", MatchPrefix+"1", 10*time.Minute)
	ms.matchData.Set(UserPrefix+"2", MatchPrefix+"1", 10*time.Minute)
	ms.matchData.Set(UserPrefix+"3", MatchPrefix+"2", 10*time.Minute)
	ms.matchData.Set(UserPrefix+"4", MatchPrefix+"2", 10*time.Minute)
	ms.matchData.Set(MatchPrefix+"1", &MatchData{IP: "127.0.0.1", Port: 9090}, 10*time.Minute)
	ms.matchData.Set(MatchPrefix+"2", &MatchData{IP: "127.0.0.2", Port: 9091}, 10*time.Minute)

	testCases := []struct {
		name         string
		userId       int
		matchFound   bool
		expectedIP   string
		expectedPort int32
	}{
		{
			name:         "match 1 player check",
			userId:       1,
			matchFound:   true,
			expectedIP:   "127.0.0.1",
			expectedPort: 9090,
		},
		{
			name:         "match 2 player check",
			userId:       3,
			matchFound:   true,
			expectedIP:   "127.0.0.2",
			expectedPort: 9091,
		},
		{
			name:       "no match player check",
			userId:     5,
			matchFound: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			jwt := jwts[test.userId-1]
			ctx := metadata.NewIncomingContext(ctxOr, metadata.Pairs("authorization", "Bearer "+jwt))

			resp, err := ms.CheckMatchmakeStatus(ctx, &mpb.CheckMatchmakeStatusRequest{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if test.matchFound {
				if !resp.Ready {
					t.Error("expected match status to be ready")
				}
				if test.expectedIP != resp.Ip {
					t.Errorf("expected ip: %v, got: %v", test.expectedIP, resp.Ip)
				}
				if test.expectedPort != resp.Port {
					t.Errorf("expected port: %v, got: %v", test.expectedPort, resp.Port)
				}
			} else {
				if resp.Ip != "" || resp.Port != 0 || resp.Ready {
					t.Errorf("not found matches should not have info in response: %v", *resp)
				}
			}
		})
	}
}

func TestCheckIfLobbyFits(t *testing.T) {
	ms := createTestMatchmakeServer()

	ms.matchData.Set(UserPrefix+"3", "1", 10*time.Minute)
	ms.matchData.Set(UserPrefix+"4", "1", 10*time.Minute)
	ms.matchData.Set(MatchPrefix+"1", &MatchData{IP: "127.0.0.1", Port: 9090}, 10*time.Minute)

	testCases := []struct {
		name             string
		playerQueue      []*PlayerData
		expectedIds      []string
		expectedDataSize int
		createdMatch     bool
	}{
		{
			name: "match created",
			playerQueue: []*PlayerData{
				{
					UserId: "5",
				},
				{
					UserId: "1",
				},
				{
					UserId: "2",
				},
			},
			expectedIds:      []string{"5", "1"},
			expectedDataSize: 6,
			createdMatch:     true,
		},
		{
			name: "match not created",
			playerQueue: []*PlayerData{
				{
					UserId: "2",
				},
			},
			expectedDataSize: 6,
			createdMatch:     false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			for _, player := range test.playerQueue {
				ms.playersMatchmaked[player.UserId] = true
			}
			ms.playersInQueue = test.playerQueue

			ms.checkIfLobbyFits(logrus.New())

			if ms.matchData.ItemCount() != test.expectedDataSize {
				t.Errorf("expected match data size to be: %v, got: %v", test.expectedDataSize, ms.matchData.ItemCount())
			}

			if test.createdMatch {
				foundIds := make([]bool, len(test.expectedIds))

				for key := range ms.matchData.Items() {
					fmt.Println(key)
					for index, expectedId := range test.expectedIds {
						if strings.TrimPrefix(key, UserPrefix) == expectedId {
							foundIds[index] = true
						}
					}
				}

				for i := range foundIds {
					if !foundIds[i] {
						t.Errorf("match value for user id %v not found", test.expectedIds[i])
					}
				}
			}
		})
	}
}
