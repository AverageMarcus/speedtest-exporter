package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/showwin/speedtest-go/speedtest"
)

var (
	latency   = time.Duration(0)
	downspeed = 0.0
	upspeed   = 0.0

	interval int
	port     int
)

func main() {
	flag.IntVar(&interval, "interval", 30, "Duration, in minutes, between speedtest runs")
	flag.IntVar(&port, "port", 9091, "The port to listen on")

	go (func() {
		for {
			checkSpeed()
			time.Sleep(time.Minute * time.Duration(interval))
		}
	})()

	collector := newSpeedCollector()
	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func checkSpeed() {
	log.Println("Performing speedtest")
	user, _ := speedtest.FetchUserInfo()
	serverList, _ := speedtest.FetchServers(user)
	targets, _ := serverList.FindServer([]int{})
	target := targets[0]
	log.Printf("Testing against server: %s - %s\n", target.Name, target.Sponsor)

	target.PingTest()
	target.DownloadTest(false)
	target.UploadTest(false)
	latency = target.Latency
	downspeed = target.DLSpeed
	upspeed = target.ULSpeed
	log.Printf("Finished speedtest. DL=%f UL=%f Ping=%v\n", downspeed, upspeed, latency)
}

type speedCollector struct {
	downMetric    *prometheus.Desc
	upMetric      *prometheus.Desc
	latencyMetric *prometheus.Desc
}

func newSpeedCollector() *speedCollector {
	return &speedCollector{
		downMetric: prometheus.NewDesc("speedtest_download_speed",
			"Download speed in Mbit/s",
			nil, nil,
		),
		upMetric: prometheus.NewDesc("speedtest_upload_speed",
			"Upload speed in Mbit/s",
			nil, nil,
		),
		latencyMetric: prometheus.NewDesc("speedtest_latency",
			"Latency in ms",
			nil, nil,
		),
	}
}

func (collector *speedCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.downMetric
	ch <- collector.upMetric
	ch <- collector.latencyMetric
}

func (collector *speedCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(collector.downMetric, prometheus.CounterValue, downspeed)
	ch <- prometheus.MustNewConstMetric(collector.upMetric, prometheus.CounterValue, upspeed)
	ch <- prometheus.MustNewConstMetric(collector.latencyMetric, prometheus.CounterValue, float64(latency.Milliseconds()))
}
