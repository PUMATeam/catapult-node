package service

import (
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/PUMATeam/catapult-node/util"
)

// TODO make configurable
const fcBridgeName = "fcbridge"

func createTapDevice(tapName string) (string, error) {
	_, err := util.ExecuteCommand("ip", "tuntap", "add", tapName, "mode", "tap")
	if err != nil {
		return "", err
	}

	return util.ExecuteCommand("ip", "link", "set", tapName, "up")
}

func addTapToBridge(tapName, bridgeName string) (string, error) {
	return util.ExecuteCommand("brctl", "addif", bridgeName, tapName)
}

func findAvailableIP() (string, error) {
	// TODO handle errors
	bridgeIP, _ := getBridgeIP()
	cmd := fmt.Sprintf("nmap -v -sn -n %s -oG - | awk '/Status: Down/{print $2}'", bridgeIP)
	out, err := util.ExecuteCommand("bash", "-c", cmd)

	if err != nil {
		log.Error(err)
		return "", err
	}

	// TOODO has to be improved and cached...
	return strings.Split(out, "\n")[1], nil
}

func getBridgeIP() (string, error) {
	iface, err := net.InterfaceByName(fcBridgeName)
	if err != nil {
		log.Error(err)
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		log.Error(err)
		return "", err
	}

	return addrs[0].String(), nil
}
