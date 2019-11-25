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
func (ns *NodeService) StartVM(ctx context.Context, cfg *node.VmConfig) (*node.VmResponse, error) {
	log.Info("Starting VM ", cfg.GetVmID().GetValue())
	vmID := cfg.GetVmID().GetValue()

	// tap device name would be fc-<last 6 characters of VM UUID>
	log.Infof("Setting up network...")
	tapDeviceName := fmt.Sprintf("%s-%s", "fc", vmID[len(vmID)-6:])
	network, err := setupNetwork(tapDeviceName)

	if err != nil {
		log.Error(err)
		return &node.VmResponse{
			Status: node.Status_FAILED,
		}, err
	}

	cfg.Address = network.ip

	fch := &fc{
		vmID:          cfg.GetVmID().GetValue(),
		tapDeviceName: tapDeviceName,
		macAddress:    network.macAddress,
		ipAddress:     network.ip,
		bridgeIP:      network.bridgeIP,
		netmask:       network.netmask,
	}

	m, err := fch.runVMM(context.Background(), cfg, log.Logger{})
	if err != nil {
		return &node.VmResponse{
			Status: node.Status_FAILED,
		}, err
	}

	ns.Machines[cfg.GetVmID().GetValue()] = m

	go fch.readPipe("log")
	go fch.readPipe("metrics")

	return &node.VmResponse{
		Status: node.Status_SUCCESS,
		Config: cfg,
	}, nil
}

func (ns *NodeService) StopVM(ctx context.Context, uuid *node.UUID) (*node.Response, error) {
	log.Debug("StopVM called on VM ", uuid.GetValue())
	v, ok := ns.Machines[uuid.GetValue()]
	if !ok {
		log.Errorf("VM %s not found", uuid.GetValue())
		return &node.Response{
			Status: node.Status_FAILED,
		}, fmt.Errorf("VM %s not found", uuid.GetValue())
	}
	err := v.StopVMM()
	if err != nil {
		log.Error("Failed to stop VM ", uuid.GetValue())
		return &node.Response{
			Status: node.Status_FAILED,
		}, err
	}

	log.Infof("Stopped VM %s", uuid.GetValue())
	log.Infof("Cleaning up...")

	vmID := uuid.GetValue()
	deleteDevice(fmt.Sprintf("%s-%s", "fc", vmID[len(vmID)-6:]))

	return &node.Response{
		Status: node.Status_FAILED,
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

func (ns *NodeService) CreateDrive(ctx context.Context, img *node.ImageName) (*node.DriveResponse, error) {
	path, err := pullImage(ctx, img.GetName())
	if err != nil {
		return &node.DriveResponse{
			Status: node.Status_FAILED,
			Size:   -1,
			Path:   "",
		}, err
	}

	path, size, err := createRootFS(path)
	if err != nil {
		log.Error(err)
		return &node.DriveResponse{
			Status: node.Status_FAILED,
			Size:   -1,
			Path:   "",
		}, err
	}

	return &node.DriveResponse{
		Status: node.Status_SUCCESS,
		Size:   size,
		Path:   path,
	}, err
}

func (ns *NodeService) ConnectVolume(ctx context.Context, vol *node.Volume) (*node.Response, error) {
	err := mapVolume(vol.GetVolumeID(), vol.GetPoolName())
	if err != nil {
		return &node.Response{
			Status: node.Status_FAILED,
		}, err
	}

	return &node.Response{
		Status: node.Status_SUCCESS,
	}, err
}
