package api

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc/keepalive"

	node "github.com/PUMATeam/catapult-node/pb"
	"github.com/PUMATeam/catapult-node/service"

	"google.golang.org/grpc"
)

const (
	logFile = "catapult-node.log"
)

var log *logrus.Logger

func init() {
	// TODO make configurable
	log = logrus.New()
	var f *os.File
	if _, err := os.Stat(logFile); err != nil {
		f, err = os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			log.Fatal(err)
		}

	}

	log.SetOutput(f)
	log.SetLevel(logrus.DebugLevel)
}

// Start starts catapult node server
func Start(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer(grpc.KeepaliveParams(
		keepalive.ServerParameters{
			Timeout: 1 * time.Minute,
		}),
	)
	nodeService := service.NewNodeService(log, make(map[string]*firecracker.Machine))
	node.RegisterNodeServer(server, nodeService)
	if err := server.Serve(lis); err != nil {
		log.Error(err)
	}
}
