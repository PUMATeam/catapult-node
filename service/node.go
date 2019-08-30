package service

import (
	"context"
	"fmt"

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
	log.Info("Starting VM", cfg.GetVmID().GetValue())
	vmID := cfg.GetVmID().GetValue()

	// tap device name would be fc-<last 6 characters of VM UUID>
	tapDeviceName := fmt.Sprintf("%s-%s", "fc", vmID[len(vmID)-6:])
	ip, bridgeIP, macAddress, err := setupNetwork(tapDeviceName)
	if err != nil {
		log.Error(err)
		return &node.Response{
			Status: node.Response_FAILED,
		}, err
	}
	cfg.Address = ip

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
		Config: cfg,
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

func (ns *NodeService) ListVMs(context.Context, *empty.Empty) (*node.VmList, error) {
	log.Debug("ListVMs called")
	vmList := new(node.VmList)
	uuid := &node.UUID{
		Value: "poop",
	}
	vmList.VmID = []*node.UUID{uuid}
	return vmList, nil
}

func setupNetwork(tapDeviceName string) (string, string, string, error) {
	bridgeIP, err := getBridgeIP()
	if err != nil {
		return "", "", "", err
	}

	_, err = createTapDevice(tapDeviceName)
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to create tap device: %s", err)
	}

	_, err = addTapToBridge(tapDeviceName, fcBridgeName)

	ip, err := findAvailableIP()
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to find IP address: %s", err)
	}

	macAddress, err := generateMACAddress()
	if err != nil {
		return "", "", "", fmt.Errorf("Failed to generate MAC address: %s", err)
	}

	return ip, bridgeIP, macAddress, nil
}
