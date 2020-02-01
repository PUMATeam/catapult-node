package service

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/PUMATeam/catapult-node/util"
)

// TODO make configurable
const fcBridgeName = "fcbridge"

var ips = make([]string, 0, 0)

type fcNetwork struct {
	ip         string
	bridgeIP   string
	netmask    string
	macAddress string
	log        *log.Logger
}

func newNetworkService(log *log.Logger) *fcNetwork {
	return &fcNetwork{
		log: log,
	}
}

func (fn *fcNetwork) createTapDevice(tapName string) (string, error) {
	_, err := util.ExecuteCommand("ip", "tuntap", "add", tapName, "mode", "tap")
	if err != nil {
		fn.log.Error("Failed to create tap device", err)
		return "", err
	}

	return util.ExecuteCommand("ip", "link", "set", tapName, "up")
}

func (fn *fcNetwork) deleteDevice(deviceName string) error {
	fn.log.Infof("Removing tap device %s", deviceName)
	_, err := util.ExecuteCommand("ip", "link", "del", deviceName)
	if err != nil {
		fn.log.Error("Failed to delete tap device", err)
		return err
	}

	return nil
}

func (fn *fcNetwork) addTapToBridge(tapName, bridgeName string) (string, error) {
	return util.ExecuteCommand("ip", "ling", "set", tapName, "master", bridgeName)
}

func (fn *fcNetwork) findAvailableIP() (string, error) {
	// TODO handle errors
	fn.log.Info("Looking for available IP address")

	bridgeIP, _ := fn.getBridge()
	if len(ips) == 0 {
		cmd := fmt.Sprintf("nmap -v -sn -n %s -oG - | awk '/Status: Down/{print $2}'",
			bridgeIP.String())
		out, err := util.ExecuteCommand("bash", "-c", cmd)
		if err != nil {
			fn.log.Errorf("Failed to execute command: %s %s", cmd, err)
			return "", err
		}

		// ignore first two addresses
		ips = strings.Split(out, "\n")[2:]
	}

	// remove selected ip
	ips = util.RemoveFromSlice(ips, 1).([]string)
	fn.log.Errorf("Found IP: %s", ips[1])
	return ips[1], nil
}

func (fn *fcNetwork) getBridge() (net.Addr, error) {
	iface, err := net.InterfaceByName(fcBridgeName)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return addrs[0], nil
}

func (fn *fcNetwork) generateMACAddress() (string, error) {
	buf := make([]byte, 6)

	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	buf[0] |= 2
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

func (fn *fcNetwork) setupNetwork(tapDeviceName string) (*fcNetwork, error) {
	bridgeAddr, err := fn.getBridge()
	if err != nil {
		return nil, err
	}

	_, bridgeIP, err := net.ParseCIDR(bridgeAddr.String())
	if err != nil {
		return nil, err
	}

	fn.log.Infof("Creating tap device %s...", tapDeviceName)
	_, err = fn.createTapDevice(tapDeviceName)
	if err != nil {
		return nil, fmt.Errorf("Failed to create tap device: %s", err)
	}

	fn.log.Infof("Adding tap device %s to bridge %s", tapDeviceName, fcBridgeName)
	_, err = fn.addTapToBridge(tapDeviceName, fcBridgeName)

	fn.log.Info("Looking for an IP address")
	ip, err := fn.findAvailableIP()
	if err != nil {
		return nil, fmt.Errorf("Failed to find IP address: %s", err)
	}

	fn.log.WithFields(log.Fields{
		"IP": ip,
	}).Info("Found IP address")

	fn.log.Info("Generating MAC address")
	macAddress, err := fn.generateMACAddress()
	if err != nil {
		return nil, fmt.Errorf("Failed to generate MAC address: %s", err)
	}
	fn.log.WithFields(log.Fields{
		"MAC": macAddress,
	}).Info("Generated MAC address")

	return &fcNetwork{
		ip: ip,
		// TODO extract and make safe
		bridgeIP:   bridgeAddr.(*net.IPNet).IP.String(),
		netmask:    net.IP(bridgeIP.Mask).String(),
		macAddress: macAddress,
	}, nil
}
