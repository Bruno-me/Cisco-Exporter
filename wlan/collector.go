package wlan

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.com/wobcom/cisco-exporter/collector"
	"gitlab.com/wobcom/cisco-exporter/connector"
)

const (
	prefix string = "cisco_wlan_"
)

var (
	interfaceNumbers = []int{0, 1}

	labelNames     = []string{"target", "interface"}
	rateLabelNames = append(labelNames, "rate")

	wlanFrequencyDesc    = prometheus.NewDesc(prefix+"frequency", "WLAN frequency information", append(labelNames, "stat"), nil)
	wlanChannelWidthDesc = prometheus.NewDesc(prefix+"channel_width", "WLAN channel bandwidth", labelNames, nil)
	wlanTxPowerDesc      = prometheus.NewDesc(prefix+"transmit_power", "WLAN radio transmit power in dBm", labelNames, nil)
	wlanStatisticsDesc   = prometheus.NewDesc(prefix+"statistics", "WLAN interface statistics counters", append(labelNames, "direction", "stat"), nil)

	wlanClientRxKilobytesDesc  = prometheus.NewDesc(prefix+"client_rx_kilobyte_count", "WLAN client receive kilobyte count", append(labelNames, "mac_address"), nil)
	wlanClientTxKilobytesDesc  = prometheus.NewDesc(prefix+"client_tx_kilobyte_count", "WLAN client transmit kilobyte count", append(labelNames, "mac_address"), nil)
	wlanClientRxPacketsDesc    = prometheus.NewDesc(prefix+"client_rx_packet_count", "WLAN client receive packet count", append(labelNames, "mac_address"), nil)
	wlanClientTxPacketsDesc    = prometheus.NewDesc(prefix+"client_tx_packet_count", "WLAN client transmit packet count", append(labelNames, "mac_address"), nil)
	wlanClientRxDuplicatesDesc = prometheus.NewDesc(prefix+"client_rx_duplicate_count", "WLAN client receive duplicate packet count", append(labelNames, "mac_address"), nil)
	wlanClientTxRetriesDesc    = prometheus.NewDesc(prefix+"client_tx_retry_count", "WLAN client transmit retry count", append(labelNames, "mac_address"), nil)
	wlanClientSNRDesc          = prometheus.NewDesc(prefix+"client_snr", "WLAN client signal-to-noise ratio", append(labelNames, "mac_address"), nil)
	wlanClientRSSIDesc         = prometheus.NewDesc(prefix+"client_rssi", "WLAN client Received Signal Strength Indicator", append(labelNames, "mac_address"), nil)

	wlanRateRxBytesDesc   = prometheus.NewDesc(prefix+"rate_rx_byte_count", "WLAN rate receive byte count", rateLabelNames, nil)
	wlanRateTxBytesDesc   = prometheus.NewDesc(prefix+"rate_tx_byte_count", "WLAN rate transmit byte count", rateLabelNames, nil)
	wlanRateRxPacketsDesc = prometheus.NewDesc(prefix+"rate_rx_packet_count", "WLAN rate receive packet count", rateLabelNames, nil)
	wlanRateTxPacketsDesc = prometheus.NewDesc(prefix+"rate_tx_packet_count", "WLAN rate transmit packet count", rateLabelNames, nil)
	wlanRateRetriesDesc   = prometheus.NewDesc(prefix+"rate_retry_count", "WLAN rate retry count", append(rateLabelNames, "type"), nil)
)

type Collector struct {
}

func NewCollector() collector.Collector {
	return &Collector{}
}

func (*Collector) Name() string {
	return "wlan"
}

func (*Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- wlanFrequencyDesc
	ch <- wlanChannelWidthDesc
	ch <- wlanTxPowerDesc
	ch <- wlanStatisticsDesc

	ch <- wlanClientRxKilobytesDesc
	ch <- wlanClientTxKilobytesDesc
	ch <- wlanClientRxPacketsDesc
	ch <- wlanClientTxPacketsDesc
	ch <- wlanClientRxDuplicatesDesc
	ch <- wlanClientTxRetriesDesc
	ch <- wlanClientSNRDesc
	ch <- wlanClientRSSIDesc

	ch <- wlanRateRxBytesDesc
	ch <- wlanRateTxBytesDesc
	ch <- wlanRateRxPacketsDesc
	ch <- wlanRateTxPacketsDesc
	ch <- wlanRateRetriesDesc
}

func (c *Collector) Collect(ctx *collector.CollectContext) {
	defer func() {
		ctx.Done <- struct{}{}
	}()

	for i := range interfaceNumbers {
		c.collectInterface(ctx, i)
	}
}

func (*Collector) collectInterface(ctx *collector.CollectContext, interfaceNum int) {
	var (
		interfaceName   = fmt.Sprintf("Dot11Radio%d", interfaceNum)
		interfaceLabels = append(ctx.LabelValues, interfaceName)
		interfaceParser = interfaceParser{
			interfaceLabels: interfaceLabels,
			metricChannel:   ctx.Metrics,
		}
	)

	command := fmt.Sprintf("show controller Dot11Radio %d", interfaceNum)
	sshCtx := connector.NewSSHCommandContext(command)
	go ctx.Connection.RunCommand(sshCtx)

	for {
		select {
		case <-sshCtx.Done:
			return
		case err := <-sshCtx.Errors:
			ctx.Errors <- errors.Wrapf(err, "Error scraping WLAN statistics: %v", err)
		case line := <-sshCtx.Output:
			interfaceParser.parseLine(line)
		}
	}
}
