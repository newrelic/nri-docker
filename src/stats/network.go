package stats

type NetworkFetcher struct {
}

type Network struct {
	RxBytes   float64
	RxDropped float64
	RxErrors  float64
	RxPackets float64
	TxBytes   float64
	TxDropped float64
	TxErrors  float64
	TxPackets float64
}

func NewNetworkFetcher() (NetworkFetcher, error) {
	return NetworkFetcher{}, nil
}

func (f *NetworkFetcher) Fetch(containerID string) (Network, error) {
	var network Network
	//for range []int{1} {
		network.RxBytes += float64(12)
		network.RxDropped += float64(23)
		network.RxErrors += float64(1234)
		network.RxPackets += float64(333)
		network.TxBytes += float64(333)
		network.TxDropped += float64(111)
		network.TxErrors += float64(111)
		network.TxPackets += float64(11)
	//}
	return network, nil
}
