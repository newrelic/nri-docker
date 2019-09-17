package stats

import (
	"bufio"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/log"
)

// TODO: use cgroups library
type NetworkFetcher struct {
}

type Network struct {
	RxBytes   int64
	RxDropped int64
	RxErrors  int64
	RxPackets int64
	TxBytes   int64
	TxDropped int64
	TxErrors  int64
	TxPackets int64
}

func NewNetworkFetcher() (NetworkFetcher, error) {
	return NetworkFetcher{}, nil
}

func getProcFolder(pid int) (string, error) {
	insideHostFile := path.Join("/proc", strconv.Itoa(pid), "net", "dev")
	insideContainerFile := path.Join(hostContainerPath, insideHostFile)
	var err error
	if _, err = os.Stat(insideContainerFile); err == nil {
		return insideContainerFile, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	if _, err := os.Stat(insideHostFile); err != nil {
		return "", err
	}
	return insideHostFile, nil
}

func (f *NetworkFetcher) Fetch(containerPid int) (Network, error) {
	var network Network
	filePath, err := getProcFolder(containerPid)
	if err != nil {
		return network, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return network, err
	}
	defer file.Close()

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
