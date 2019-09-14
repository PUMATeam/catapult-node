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

func createTapDevice(tapName string) (string, error) {
	_, err := util.ExecuteCommand("ip", "tuntap", "add", tapName, "mode", "tap")
	if err != nil {
		return "", err
	}

	return util.ExecuteCommand("ip", "link", "set", tapName, "up")
}

func deleteDevice(tapName string) error {
	_, err := util.ExecuteCommand("ip", "link", "del", tapName)
	if err != nil {
		return err
	}

	return nil
}

func addTapToBridge(tapName, bridgeName string) (string, error) {
	return util.ExecuteCommand("brctl", "addif", bridgeName, tapName)
}

func findAvailableIP() (string, error) {
	// TODO handle errors
	bridgeIP, _ := getBridge()
	if len(ips) == 0 {
		cmd := fmt.Sprintf("nmap -v -sn -n %s -oG - | awk '/Status: Down/{print $2}'",
			bridgeIP.String())
		out, err := util.ExecuteCommand("bash", "-c", cmd)
		if err != nil {
			log.Error(err)
			return "", err
		}

		// ignore first to addresses
		ips = strings.Split(out, "\n")[2:]
	}

	// remove selected ip
	ips = util.RemoveFromSlice(ips, 1).([]string)

	return ips[1], nil
}

func getBridge() (net.Addr, error) {
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

func generateMACAddress() (string, error) {
	buf := make([]byte, 6)

	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	buf[0] |= 2
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}
