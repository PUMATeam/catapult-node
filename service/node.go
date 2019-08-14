package service

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	node "github.com/PUMATeam/catapult-node/pb"

	"github.com/golang/protobuf/ptypes/empty"
)

type NodeService struct {
}

// StartVM starts a firecracker VM with the provided configuration
func (ns *NodeService) StartVM(ctx context.Context, cfg *node.VmConfig) (*node.Response, error) {
	log.Debug("StartVM called cfg: ", cfg)
	fch := &fcHandler{
		vmID: cfg.GetVmID().Value,
	}
	errChan := make(chan error, 1)

	go func() {
		errChan <- fch.runVMM(context.Background(), cfg, log.Logger{})
	}()

	var err error

	select {
	case err = <-errChan:
		if err != nil {
			log.Error("error ", err)
			return &node.Response{
				Status: node.Response_FAILED,
			}, err
		}
	case <-time.After(30 * time.Second):
		log.Info("No error return after 30 seconds, assuming success")
	}

	go fch.readPipe("log")
	go fch.readPipe("metrics")

	return &node.Response{
		Status: node.Response_SUCCESSFUL,
	}, nil
}

func (ns *NodeService) StopVM(context.Context, *node.UUID) (*node.Response, error) {
	log.Debug("StopVM called")
	return &node.Response{
		Status: node.Response_SUCCESSFUL,
	}, nil
}

func (ns *NodeService) ListVMs(context.Context, *empty.Empty) (*node.VmList, error) {
	log.Debug("ListVMs called")
	vmList := new(node.VmList)
	uuid := &node.UUID{
		Value: "poop",
	}
	vmList.VmID = uuid
	return vmList, nil
}
