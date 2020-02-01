package service

import (
	"context"
	"fmt"

	"github.com/firecracker-microvm/firecracker-go-sdk"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	node "github.com/PUMATeam/catapult-node/pb"

	"github.com/golang/protobuf/ptypes/empty"
)

type NodeService struct {
	Machines map[string]*firecracker.Machine
	log      *logrus.Logger
	storage  *storage
}

func NewNodeService(log *logrus.Logger, machines map[string]*firecracker.Machine) *NodeService {
	return &NodeService{
		Machines: machines,
		log:      log,
		storage:  &storage{log: log},
	}
}

// StartVM starts a firecracker VM with the provided configuration
func (ns *NodeService) StartVM(ctx context.Context, cfg *node.VmConfig) (*node.VmResponse, error) {
	ns.log.Info("Starting VM ", cfg.GetVmID().GetValue())
	vmID := cfg.GetVmID().GetValue()

	fcNetwork := newNetworkService(ns.log)
	// tap device name would be fc-<last 6 characters of VM UUID>
	ns.log.Infof("Setting up network...")
	tapDeviceName := fmt.Sprintf("%s-%s", "fc", vmID[len(vmID)-6:])
	network, err := fcNetwork.setupNetwork(tapDeviceName)

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

	ns.log.Infof("Starting VM ")
	m, err := fch.runVMM(context.Background(), cfg, ns.log)
	if err != nil {
		return &node.VmResponse{
			Status: node.Status_FAILED,
		}, err
	}

	ns.Machines[cfg.GetVmID().GetValue()] = m

	go fch.readPipe(ns.log, "log")
	go fch.readPipe(ns.log, "metrics")

	return &node.VmResponse{
		Status: node.Status_SUCCESS,
		Config: cfg,
	}, nil
}

func (ns *NodeService) StopVM(ctx context.Context, uuid *node.UUID) (*node.Response, error) {
	ns.log.Debug("StopVM called on VM ", uuid.GetValue())
	v, ok := ns.Machines[uuid.GetValue()]
	if !ok {
		ns.log.Errorf("VM %s not found", uuid.GetValue())
		return &node.Response{
			Status: node.Status_FAILED,
		}, fmt.Errorf("VM %s not found", uuid.GetValue())
	}
	err := v.StopVMM()
	if err != nil {
		ns.log.Errorf("Failed to stop VM %s", uuid.GetValue())
		return &node.Response{
			Status: node.Status_FAILED,
		}, err
	}

	ns.log.Infof("Stopped VM %s", uuid.GetValue())
	ns.log.Info("Cleaning up...")

	vmID := uuid.GetValue()
	fcNetwork := newNetworkService(ns.log)
	fcNetwork.deleteDevice(fmt.Sprintf("%s-%s", "fc", vmID[len(vmID)-6:]))

	return &node.Response{
		Status: node.Status_FAILED,
	}, nil
}

func (ns *NodeService) ListVMs(context.Context, *empty.Empty) (*node.VmList, error) {
	ns.log.Debug("ListVMs called")
	vmList := new(node.VmList)
	uuid := &node.UUID{
		Value: "poop",
	}

	vmList.VmID = []*node.UUID{uuid}
	return vmList, nil
}

func (ns *NodeService) CreateDrive(ctx context.Context, img *node.ImageName) (*node.DriveResponse, error) {
	path, err := ns.storage.pullImage(ctx, img.GetName())
	if err != nil {
		return &node.DriveResponse{
			Status: node.Status_FAILED,
			Size:   -1,
			Path:   "",
		}, err
	}

	path, size, err := ns.storage.createRootFS(path)
	if err != nil {
		ns.log.Error(err)
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
	err := ns.storage.mapVolume(vol.GetVolumeID(), vol.GetPoolName())
	if err != nil {
		return &node.Response{
			Status: node.Status_FAILED,
		}, err
	}

	return &node.Response{
		Status: node.Status_SUCCESS,
	}, err
}
