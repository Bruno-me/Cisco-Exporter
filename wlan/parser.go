package wlan

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"gitlab.com/wobcom/cisco-exporter/util"
)

const (
	dot11StatsHeader     = `	DOT11 Statistics (Cumulative Total/Last 5 Seconds):`
	clientStatsHeader    = `                 RxPkts KBytes  Dup Dec Mic Txc  TxPkts  KBytes  Retry RSSI SNR Fail BAfail KeySetTs KeyClearTs1 Ts2 key`
	clientStatsTrailer   = `              Agr TxLt   TxRP   St ACQ/TW  TxACQ  Stats`
	noClientStatsTrailer = `               (Client) MaxPri DefUniPri DefMultPri WiredProt`
)

var (
	configuredChannelRegex = regexp.MustCompile(`^Configured Frequency: (\d+) MHz\s+Channel (\d+)( \(DFS enabled\))?`)
	servingChannelRegex    = regexp.MustCompile(`^Serving Frequency: (\d+) MHz\s+Channel (\d+)\s+(\d+)MHz`)
	txPowerRegex           = regexp.MustCompile(`^Configured TxPower:\s+(\d+) dBm \(Level Index \d+\)$`)

	columnKVRegex = regexp.MustCompile(`^([a-zA-Z0-9 ]+):\s*(\d+) / \d+\s+([a-zA-Z0-9 >]+):\s*(\d+) .*$`)

	rateRegex        = regexp.MustCompile(`^RATE\s+([0-9.am-]+)`)
	ratePacketRegex  = regexp.MustCompile(`^Rx Packets:\s+(\d+) / \d+\s+Tx Packets:\s+(\d+) .*$`)
	rateBytesRegex   = regexp.MustCompile(`^Rx Bytes:\s+(\d+) / \d+\s+Tx Bytes:\s+(\d+) .*$`)
	rateRetriesRegex = regexp.MustCompile(`^RTS Retries:\s+(\d+) / \d+\s+Data Retries:\s+(\d+) .*$`)

	interfaceRxStatMap = map[string]string{
		"Beacons Rx":           "beacons",
		"Broadcasts Rx":        "broadcasts",
		"Broadcasts to host":   "broadcasts_host",
		"Buffer full":          "buffer_full",
		"CRC errors":           "crc_errors",
		"Duplicate frames":     "duplicates",
		"Header CRC errors":    "header_crc_errors",
		"Host Rx K Bytes":      "host_kilobytes",
		"Host buffer full":     "host_buffer_full",
		"Incomplete fragments": "incomplete_fragments",
		"Invalid header":       "invalid_header",
		"Length invalid":       "length_invalid",
		"Mgmt Packets Rx":      "management_packets",
		"Multicasts Rx":        "multicasts",
		"Multicasts to host":   "multicasts_host",
		"RTS received":         "rts",
		"Rx Concats":           "rx_concats",
		"Unicasts Rx":          "unicasts",
		"Unicasts to host":     "unicasts_host",
		"WEP errors":           "wep_errors",
	}
	interfaceTxStatMap = map[string]string{
		"Beacons Tx":           "beacons",
		"Broadcasts Tx":        "broadcasts",
		"Broadcasts by host":   "broadcasts_host",
		"CTS not received":     "no_cts",
		"Energy detect defers": "energy_detect_defers",
		"Host Tx K Bytes":      "host_kilobytes",
		"Jammer detected":      "jammer_detected",
		"Mgmt Packets Tx":      "management_packets",
		"Multicasts Tx":        "multicasts",
		"Multicasts by host":   "multicasts_host",
		"Packets > 1 retry":    "multiple_retries",
		"Packets aged":         "aged",
		"Packets one retry":    "one_retry",
		"Protocol defers":      "protocol_defers",
		"RTS transmitted":      "rts",
		"Retries":              "retries",
		"Tx Concats":           "concats",
		"Unicast Fragments Tx": "unicast_fragments",
		"Unicasts Tx":          "unicasts",
		"Unicasts by host":     "unicasts_host",
	}
)

type interfaceParser struct {
	interfaceLabels []string
	metricChannel   chan<- prometheus.Metric

	configuredChannelSent bool
	servingChannelSent    bool
	clientStatsFound      bool
	clientStatsSent       bool
	txPowerSent           bool
	dot11StatsFound       bool
	dot11StatsSent        bool
	currentRate           *string
}

func (parser *interfaceParser) parseLine(line string) {
	if !parser.configuredChannelSent && configuredChannelRegex.MatchString(line) {
		matches := configuredChannelRegex.FindStringSubmatch(line)
		frequency, _ := strconv.ParseFloat(matches[1], 64)

		labels := append(parser.interfaceLabels, "configured")

		parser.metricChannel <- prometheus.MustNewConstMetric(wlanFrequencyDesc, prometheus.GaugeValue, frequency, labels...)

		parser.configuredChannelSent = true
	} else if !parser.servingChannelSent && servingChannelRegex.MatchString(line) {
		matches := servingChannelRegex.FindStringSubmatch(line)
		frequency, _ := strconv.ParseFloat(matches[1], 64)
		width, _ := strconv.ParseFloat(matches[3], 64)

		freqLabels := append(parser.interfaceLabels, "serving")

		parser.metricChannel <- prometheus.MustNewConstMetric(wlanFrequencyDesc, prometheus.GaugeValue, frequency, freqLabels...)
		parser.metricChannel <- prometheus.MustNewConstMetric(wlanChannelWidthDesc, prometheus.GaugeValue, width, parser.interfaceLabels...)

		parser.servingChannelSent = true
	} else if !parser.txPowerSent && txPowerRegex.MatchString(line) {
		matches := txPowerRegex.FindStringSubmatch(line)
		power, _ := strconv.ParseFloat(matches[1], 64)

		parser.metricChannel <- prometheus.MustNewConstMetric(wlanTxPowerDesc, prometheus.GaugeValue, power, parser.interfaceLabels...)

		parser.txPowerSent = true
	} else if parser.clientStatsFound && !parser.clientStatsSent {
		if line == clientStatsTrailer || line == noClientStatsTrailer {
			parser.clientStatsSent = true
		} else {
			fields := strings.Fields(line)
			labels := append(parser.interfaceLabels, fields[0])

			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientRxPacketsDesc, prometheus.CounterValue, util.Str2float64(fields[1]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientRxKilobytesDesc, prometheus.CounterValue, util.Str2float64(fields[2]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientRxDuplicatesDesc, prometheus.CounterValue, util.Str2float64(fields[3]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientTxPacketsDesc, prometheus.CounterValue, util.Str2float64(fields[7]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientTxKilobytesDesc, prometheus.CounterValue, util.Str2float64(fields[8]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientTxRetriesDesc, prometheus.CounterValue, util.Str2float64(fields[9]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientRSSIDesc, prometheus.GaugeValue, -util.Str2float64(fields[10]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanClientSNRDesc, prometheus.GaugeValue, util.Str2float64(fields[11]), labels...)
		}
	} else if !parser.clientStatsFound && !parser.clientStatsSent && line == clientStatsHeader {
		parser.clientStatsFound = true
	} else if parser.dot11StatsFound && !parser.dot11StatsSent {
		if line == "" {
			parser.dot11StatsSent = true
		} else if matches := columnKVRegex.FindStringSubmatch(line); matches != nil {
			rxLabels := append(parser.interfaceLabels, "rx", interfaceRxStatMap[matches[1]])
			txLabels := append(parser.interfaceLabels, "tx", interfaceTxStatMap[matches[3]])

			parser.metricChannel <- prometheus.MustNewConstMetric(wlanStatisticsDesc, prometheus.CounterValue, util.Str2float64(matches[2]), rxLabels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanStatisticsDesc, prometheus.CounterValue, util.Str2float64(matches[4]), txLabels...)
		}
	} else if !parser.dot11StatsFound && !parser.dot11StatsSent && line == dot11StatsHeader {
		parser.dot11StatsFound = true
	} else if parser.currentRate != nil {
		if line == "" {
			parser.currentRate = nil
		}
		if matches := rateBytesRegex.FindStringSubmatch(line); matches != nil {
			labels := append(parser.interfaceLabels, *parser.currentRate)

			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateRxBytesDesc, prometheus.CounterValue, util.Str2float64(matches[1]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateTxBytesDesc, prometheus.CounterValue, util.Str2float64(matches[2]), labels...)
		} else if matches := ratePacketRegex.FindStringSubmatch(line); matches != nil {
			labels := append(parser.interfaceLabels, *parser.currentRate)

			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateRxPacketsDesc, prometheus.CounterValue, util.Str2float64(matches[1]), labels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateTxPacketsDesc, prometheus.CounterValue, util.Str2float64(matches[2]), labels...)
		} else if matches := rateRetriesRegex.FindStringSubmatch(line); matches != nil {
			rtsLabels := append(parser.interfaceLabels, *parser.currentRate, "rts")
			dataLabels := append(parser.interfaceLabels, *parser.currentRate, "data")

			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateRetriesDesc, prometheus.CounterValue, util.Str2float64(matches[1]), rtsLabels...)
			parser.metricChannel <- prometheus.MustNewConstMetric(wlanRateRetriesDesc, prometheus.CounterValue, util.Str2float64(matches[2]), dataLabels...)
		}
	} else if parser.dot11StatsSent && parser.currentRate == nil && rateRegex.MatchString(line) {
		matches := rateRegex.FindStringSubmatch(line)
		parser.currentRate = &matches[1]
	}
}
