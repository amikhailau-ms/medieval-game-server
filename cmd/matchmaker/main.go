package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/amikhailau/medieval-game-server/pkg/mpb"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/infobloxopen/atlas-app-toolkit/gateway"
	"github.com/infobloxopen/atlas-app-toolkit/requestid"
	"github.com/infobloxopen/atlas-app-toolkit/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/runtime/protoiface"
)

func main() {
	doneC := make(chan error)
	logger := NewLogger()

	go func() { doneC <- ServeExternal(logger) }()

	if err := <-doneC; err != nil {
		logger.Fatal(err)
	}
}

func NewLogger() *logrus.Logger {
	logger := logrus.StandardLogger()
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Set the log level on the default logger based on command line flag
	logLevels := map[string]logrus.Level{
		"debug":   logrus.DebugLevel,
		"info":    logrus.InfoLevel,
		"warning": logrus.WarnLevel,
		"error":   logrus.ErrorLevel,
		"fatal":   logrus.FatalLevel,
		"panic":   logrus.PanicLevel,
	}
	if level, ok := logLevels[viper.GetString("matchmaker.logging.level")]; !ok {
		logger.Errorf("Invalid %q provided for log level", viper.GetString("matchmaker.logging.level"))
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(level)
	}

	return logger
}

// ServeExternal builds and runs the server that listens on ServerAddress and GatewayAddress
func ServeExternal(logger *logrus.Logger) error {

	grpcServer, err := NewGRPCServer(logger)
	if err != nil {
		logger.Fatalln(err)
	}

	s, err := server.NewServer(
		server.WithGrpcServer(grpcServer),
		server.WithGateway(
			gateway.WithGatewayOptions(
				runtime.WithForwardResponseOption(forwardResponseOption),
				runtime.WithIncomingHeaderMatcher(gateway.ExtendedDefaultHeaderMatcher(
					requestid.DefaultRequestIDKey)),
			),
			gateway.WithServerAddress(fmt.Sprintf("%s:%s", viper.GetString("matchmaker.server.address"), viper.GetString("matchmaker.server.port"))),
			gateway.WithEndpointRegistration(viper.GetString("matchmaker.gateway.endpoint"), mpb.RegisterMatchmakerHandlerFromEndpoint),
		),
		server.WithHandler("/swagger/", NewSwaggerHandler(viper.GetString("matchmaker.gateway.swaggerFile"))),
	)
	if err != nil {
		logger.Fatalln(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = viper.GetString("matchmaker.gateway.port")
	}

	grpcL, err := net.Listen("tcp", fmt.Sprintf("%s:%s", viper.GetString("matchmaker.server.address"), viper.GetString("matchmaker.server.port")))
	if err != nil {
		logger.Fatalln(err)
	}

	httpL, err := net.Listen("tcp", fmt.Sprintf("%s:%s", viper.GetString("matchmaker.gateway.address"), port))
	if err != nil {
		logger.Fatalln(err)
	}

	logger.Printf("serving gRPC at %s:%s", viper.GetString("matchmaker.server.address"), viper.GetString("matchmaker.server.port"))
	logger.Printf("serving http at %s:%s", viper.GetString("matchmaker.gateway.address"), port)

	return s.Serve(grpcL, httpL)
}

func init() {
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AddConfigPath(viper.GetString("matchmaker.config.source"))
	if viper.GetString("matchmaker.config.file") != "" {
		log.Printf("Serving from configuration file: %s", viper.GetString("matchmaker.config.file"))
		viper.SetConfigName(viper.GetString("matchmaker.config.file"))
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("cannot load configuration: %v", err)
		}
	} else {
		log.Printf("Serving from default values, environment variables, and/or flags")
	}
}

func forwardResponseOption(ctx context.Context, w http.ResponseWriter, resp protoiface.MessageV1) error {
	w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
	return nil
}

func NewSwaggerHandler(swaggerDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/swagger/", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, fmt.Sprintf("%s/%s", swaggerDir, strings.TrimPrefix(request.URL.Path, "/swagger/")))
	})

	return mux
}
