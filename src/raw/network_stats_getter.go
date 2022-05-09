package raw

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/log"
)

type NetworkStatsGetter interface {
	GetForContainer(hostRoot, pid, containerID string) (Network, error)
}

type NetDevNetworkStatsGetter struct{}

func (cd NetDevNetworkStatsGetter) GetForContainer(hostRoot, pid, containerID string) (Network, error) {
	netMetricsPath := filepath.Join(hostRoot, "/proc", pid, "net", "dev")
	net, err := cd.network(netMetricsPath)
	if err != nil {
		log.Error(
			"couldn't fetch network stats for container %s from cgroups: %v",
			containerID,
			err,
		)
	}
	return net, err
}

// network fetches the network metrics from the /proc file system
// TODO: use cgroups library + split this to open and parse.
func (cd NetDevNetworkStatsGetter) network(filePath string) (Network, error) {
	var network Network
	file, err := os.Open(filePath)
	if err != nil {
		return network, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Error("Failed to close file: %s, error: %v", filePath, err)
		}
	}()

	sc := bufio.NewScanner(file)
	sc.Split(bufio.ScanLines)
	sc.Scan() // scan first header line
	sc.Scan() // scan second header line
	for sc.Scan() {
		ws := bufio.NewScanner(strings.NewReader(sc.Text()))
		ws.Split(bufio.ScanWords)
		words := make([]string, 0, 18)
		for ws.Scan() {
			words = append(words, ws.Text())
		}
		if len(words) < 13 {
			log.Debug("apparently malformed line: %s", sc.Text())
			continue
		}
		if strings.HasPrefix(words[0], "lo") { // ignoring loopback
			continue
		}

		rxBytes, err := strconv.Atoi(words[1])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		rxPackets, err := strconv.Atoi(words[2])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		rxErrors, err := strconv.Atoi(words[3])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		rxDropped, err := strconv.Atoi(words[4])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		txBytes, err := strconv.Atoi(words[9])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		txPackets, err := strconv.Atoi(words[10])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		txErrors, err := strconv.Atoi(words[11])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}
		txDropped, err := strconv.Atoi(words[12])
		if err != nil {
			log.Debug("apparently malformed line %q. Cause: %s", sc.Text(), err.Error())
			continue
		}

		// we are computing the sum between all network interfaces
		network.RxBytes += int64(rxBytes)
		network.RxDropped += int64(rxDropped)
		network.RxErrors += int64(rxErrors)
		network.RxPackets += int64(rxPackets)
		network.TxBytes += int64(txBytes)
		network.TxDropped += int64(txDropped)
		network.TxErrors += int64(txErrors)
		network.TxPackets += int64(txPackets)
	}

	return network, nil
}
