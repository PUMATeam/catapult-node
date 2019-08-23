package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/firecracker-microvm/firecracker-go-sdk"

	log "github.com/sirupsen/logrus"

	node "github.com/PUMATeam/catapult-node/pb"

	"github.com/golang/protobuf/ptypes/empty"
)

type NodeService struct {
	Machines map[string]*firecracker.Machine
}

// StartVM starts a firecracker VM with the provided configuration
func (ns *NodeService) StartVM(ctx context.Context, cfg *node.VmConfig) (*node.Response, error) {
	log.Debug("Starting VM", cfg.GetVmID().GetValue())

	// TOOD has to be improved....
	vmID := strconv.Itoa(rand.Intn(10))
	tapDeviceName := fmt.Sprintf("%s-%s", "fc", vmID)
	bridgeIP, err := getBridgeIP()
	log.Infof("bridge IP %s", bridgeIP)
	if err != nil {
		log.Error("Failed to get bridge ip ", err)
		return &node.Response{
			Status: node.Response_FAILED,
		}, err
	}

	_, err = createTapDevice(tapDeviceName)
	if err != nil {
		log.Error("Failed to create tap device ", err)
		return &node.Response{
			Status: node.Response_FAILED,
		}, err

	}

	_, err = addTapToBridge(tapDeviceName, fcBridgeName)

	ip, err := findAvailableIP()
	if err != nil {
		log.Error("Failed to find IP address ", err)
		return &node.Response{
			Status: node.Response_FAILED,
		}, err
	}

	log.Infof("Found available IP %s", ip)

	// TODO major improvements required
	macAddress := "02:FC:00:00:00:0" + vmID
	log.Infof("mac address for vm: %s", macAddress)
	fch := &fcHandler{
		vmID:          cfg.GetVmID().GetValue(),
		tapDeviceName: tapDeviceName,
		macAddress:    macAddress,
		ipAddress:     ip,
		bridgeIP:      bridgeIP,
	}

	m, err := fch.runVMM(context.Background(), cfg, log.Logger{})
	if err != nil {
		return &node.Response{
			Status: node.Response_FAILED,
		}, err
	}

	ns.Machines[cfg.GetVmID().GetValue()] = m

	go fch.readPipe("log")
	go fch.readPipe("metrics")

	return &node.Response{
		Status: node.Response_SUCCESSFUL,
	}, nil
}

func (ns *NodeService) StopVM(ctx context.Context, uuid *node.UUID) (*node.Response, error) {
	log.Debug("StopVM called on VM ", uuid.GetValue())
	if v, ok := ns.Machines[uuid.GetValue()]; !ok {
		log.Errorf("VM %s not found", uuid.GetValue())
		return &node.Response{
			Status: node.Response_FAILED,
		}, fmt.Errorf("VM %s not found", uuid.GetValue())
	} else {
		err := v.StopVMM()
		if err != nil {
			log.Error("Failed to stop VM ", uuid.GetValue())
			return &node.Response{
				Status: node.Response_FAILED,
			}, err
		}
	}

	log.Infof("Stopped VM %s", uuid.GetValue())

	return &node.Response{
		Status: node.Response_SUCCESSFUL,
	}, nil
}

func (ns *NodeService) ListVMs(context.Context, *empty.Empty) (*VmList, error) {
	log.Debug("ListVMs called")
	vmList := new(node.VmList)
	uuid := &node.UUID{
		Value: "poop",
	}
	vmList.VmID = []*node.UUID{uuid}
	return vmList, nil
}
