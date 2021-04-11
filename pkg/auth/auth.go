package auth

import (
	"context"
	"time"

	"github.com/dgrijalva/jwt-go"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		logger := ctxlogrus.Extract(ctx)
		logger.Debug("Authorization interceptor")

		logger.Debug("Checking claims")
		claims, err := GetAuthorizationData(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "Authorization failed - invalid header/token")
		}
		if claims.ExpiresAt < time.Now().Unix() {
			return nil, status.Error(codes.Unauthenticated, "Authorization failed - token expired")
		}
		logger.WithField("claims", claims).Debug("Incoming claims")

		return handler(ctx, req)
	}
}

func GetAuthorizationData(ctx context.Context) (*GameClaims, error) {
	logger := ctxlogrus.Extract(ctx)
	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		logger.WithError(err).Error("Token not found")
		return nil, err
	}
	claims := &GameClaims{}
	parser := &jwt.Parser{}
	_, _, err = parser.ParseUnverified(token, claims)
	if err != nil {
		logger.WithError(err).Error("Not able to parse token")
		return nil, err
	}
	return claims, nil
}
