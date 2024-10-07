package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"log/slog"
	"net/http"
	"os"
)

var (
	namespace = "adguardhome"

	tr = http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client = http.Client{Transport: &tr}

	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Exporter status.",
		nil, nil,
	)
	upstreamTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "upstream_responses"),
		"Upstreams average response time (in seconds).",
		[]string{"address"}, nil,
	)
	dnsQueries = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "dns_queries"),
		"Total number of DNS queries.",
		nil, nil,
	)
	blockedDNSqueries = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "blocked_dns_queries"),
		"Total number of blocked DNS queries.",
		nil, nil,
	)
	processingTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "processing_time"),
		"Average DNS query processing time (in seconds).",
		nil, nil,
	)
	safeBrowsing = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "blocked_safe_browsing"),
		"Blocked requests via Safe Browsing.",
		nil, nil,
	)
	safeSearch = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "blocked_safe_search"),
		"Blocked requests via Safe Search.",
		nil, nil,
	)
)

type Response struct {
	UpstreamTime      []map[string]float64 `json:"top_upstreams_avg_time"`
	AllDNSQueries     int                  `json:"num_dns_queries"`
	BlockedDNSQueries int                  `json:"num_blocked_filtering"`
	ProcessingTime    float64              `json:"avg_processing_time"`
	SafeBrowsing      int                  `json:"num_replaced_safebrowsing"`
	SafeSearch        int                  `json:"num_replaced_safesearch"`
}

type Exporter struct {
	Endpoint, Username, Password string
}

func NewExporter(endpoint, username, password string) *Exporter {
	return &Exporter{
		Endpoint: endpoint,
		Username: username,
		Password: password,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- upstreamTime
	ch <- dnsQueries
	ch <- blockedDNSqueries
	ch <- processingTime
	ch <- safeBrowsing
	ch <- safeSearch
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	err := e.CollectFromAPI(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		fmt.Printf("ERROR: %v", err)
	}

	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)
}

func (e *Exporter) CollectFromAPI(ch chan<- prometheus.Metric) error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%v/control/stats", e.Endpoint), nil)
	if err != nil {
		return err
	}

	header := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v:%v", e.Username, e.Password)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %v", header))

	response, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var res Response
	err = json.Unmarshal(body, &res)

	for _, i := range res.UpstreamTime {
		for k, v := range i {
			ch <- prometheus.MustNewConstMetric(
				upstreamTime, prometheus.GaugeValue, float64(v), k,
			)
		}
	}

	ch <- prometheus.MustNewConstMetric(
		dnsQueries, prometheus.GaugeValue, float64(res.AllDNSQueries),
	)
	ch <- prometheus.MustNewConstMetric(
		blockedDNSqueries, prometheus.GaugeValue, float64(res.BlockedDNSQueries),
	)
	ch <- prometheus.MustNewConstMetric(
		processingTime, prometheus.GaugeValue, res.ProcessingTime,
	)
	ch <- prometheus.MustNewConstMetric(
		safeBrowsing, prometheus.GaugeValue, float64(res.SafeBrowsing),
	)
	ch <- prometheus.MustNewConstMetric(
		safeSearch, prometheus.GaugeValue, float64(res.SafeSearch),
	)

	return nil
}

func main() {

	// flags
	endpoint := flag.String("endpoint", "",
		"Adguard endpoint")
	username := flag.String("username", "",
		"Username")
	password := flag.String("password", "",
		"Password")
	address := flag.String("address", ":8000",
		"Address on which to expose metrics")
	path := flag.String("path", "/metrics",
		"Metrics path (/path)")

	// check env
	config := map[string]*string{
		"ADGUARD_ENDPOINT": endpoint,
		"ADGUARD_USERNAME": username,
		"ADGUARD_PASSWORD": password,
		"ADGUARD_ADDRESS":  address,
		"ADGUARD_PATH":     path,
	}

	for key, value := range config {
		if envValue := os.Getenv(key); envValue != "" {
			*value = envValue
		}
	}

	flag.Parse()

	exporter := NewExporter(*endpoint, *username, *password)
	r := prometheus.NewRegistry()
	r.MustRegister(exporter)

	http.Handle(*path, promhttp.HandlerFor(r, promhttp.HandlerOpts{}))
	slog.Info(fmt.Sprintf("Listening on %v%v", *address, *path))
	http.ListenAndServe(*address, nil)
}
