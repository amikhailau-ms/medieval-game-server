package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/amikhailau/medieval-game-server/pkg/auth"
	"github.com/amikhailau/medieval-game-server/pkg/mpb"
	"github.com/amikhailau/medieval-game-server/pkg/pb"
	"github.com/dgrijalva/jwt-go"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/infobloxopen/atlas-app-toolkit/requestid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	matchmakerEndpoint = "/matchmake"
	userIDHeader       = "User-ID"
)

var (
	jwts              = []string{}
	matchmakerAddress = "192.168.99.145"
	matchmakerPort    = "31646"
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
	}
}

func main() {

	logger := logrus.New()
	if len(os.Args) > 2 {
		matchmakerAddress = os.Args[1]
		matchmakerPort = os.Args[2]
	}

	var waitGroup sync.WaitGroup

	for id := 0; id < 5; id++ {
		logEntry := logrus.NewEntry(logger)
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		waitGroup.Add(1)
		go dummyClient(logEntry, client, &waitGroup, id)
	}

	waitGroup.Add(-1)
	waitGroup.Wait()
}

func dummyClient(logger *logrus.Entry, client *http.Client, wg *sync.WaitGroup, id int) {
	logger.WithField("dummy-client-id", id)

	body, err := json.Marshal(&mpb.MatchmakeRequest{})
	if err != nil {
		logger.WithError(err).Error("dummy client failed")
		return
	}

	_, err = HttpRequest(logger, client, http.MethodPost, matchmakerAddress+":"+matchmakerPort+matchmakerEndpoint,
		jwts[id], "", fmt.Sprintf("dummy-client-%d", id), bytes.NewBuffer(body))
	if err != nil {
		logger.WithError(err).Error("dummy client failed")
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	var connectionData mpb.CheckMatchmakeStatusResponse
	for {
		resp, err := HttpRequest(logger, client, http.MethodGet, matchmakerAddress+":"+matchmakerPort+matchmakerEndpoint,
			jwts[id], "", fmt.Sprintf("dummy-client-%d", id), nil)

		err = jsonpb.Unmarshal(bytes.NewBuffer(resp), &connectionData)
		if err != nil {
			logger.WithError(err).Error("dummy client failed")
			return
		}

		logger.Infof("Received data: %v", connectionData)

		if connectionData.Ready {
			break
		} else {
			<-ticker.C
		}
	}
	ticker.Stop()

	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", connectionData.Ip, connectionData.Port), opts...)
	if err != nil {
		logger.WithError(err).Error("dummy client failed")
		return
	}

	grpcClient := pb.NewGameManagerClient(conn)
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(userIDHeader, strconv.Itoa(id)),
	)
	resp, err := grpcClient.Connect(ctx, &pb.ConnectRequest{
		UserId:    strconv.Itoa(id),
		LocalTime: ptypes.TimestampNow(),
	})
	if err != nil {
		logger.WithError(err).Error("dummy client failed")
		return
	}

	logger.Infof("Response from server:\n\tClientToken: %v\n\tPing: %v\n", resp.Token, resp.Ping)
	wg.Done()
}

func HttpRequest(logger *logrus.Entry, client *http.Client, method, url, token, tokenType, reqId string, body io.Reader) (res []byte, err error) {
	logger = logger.WithField("url", url)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		logger.WithError(err).Error("Failed to create new request")
		return nil, err
	}

	if len(tokenType) == 0 {
		tokenType = "Bearer"
	}
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, token))
	if reqId != "" {
		req.Header.Set(requestid.DefaultRequestIDKey, reqId)
		req.Header.Set(requestid.DeprecatedRequestIDKey, reqId)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Error("Failed to call matchmaker")
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Error("Failed to read the response")
		return nil, err
	}

	if resp.StatusCode == 404 {
		logger.WithError(err).WithField("code", resp.StatusCode).Error("Failed to read the response")
		return nil, status.Errorf(codes.NotFound, ": %s - %s", resp.Status, string(respBody))
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		logger.WithError(err).WithField("code", resp.StatusCode).Error("Failed to read the response")
		return nil, fmt.Errorf("HttpRequest failed with status: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}
