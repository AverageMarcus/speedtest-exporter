package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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
	serverID int
	cutOff   float64
)

func init() {
	flag.IntVar(&interval, "interval", 30, "Duration, in minutes, between speedtest runs")
	flag.IntVar(&port, "port", 9091, "The port to listen on")
	flag.IntVar(&serverID, "server", 55137, "The ID of the server to test against")
	flag.Float64Var(&cutOff, "cut-off", 1024, "The upper limit expected from speed test. Values above this are treated as invalid")
	flag.Parse()
}

func main() {
	go (func() {
		for {
			if err := checkSpeed(); err != nil {
				// Retry after a minute on error
				time.Sleep(time.Minute)
			} else {
				time.Sleep(time.Minute * time.Duration(interval))
			}
		}
	})()

	collector := newSpeedCollector()
	prometheus.MustRegister(collector)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func checkSpeed() error {
	log.Println("Performing speedtest")
	user, err := speedtest.FetchUserInfo()
	if err != nil {
		log.Printf("Error fetching user info: %v\n", err)
		return err
	}
	serverList, err := speedtest.FetchServers(user)
	if err != nil {
		log.Printf("Error fetching server list: %v\n", err)
		return err
	}
	targets, err := serverList.FindServer([]int{serverID})
	if err != nil {
		log.Printf("Error finding server: %v\n", err)
		return err
	}
	if len(targets) == 0 {
		log.Printf("No servers found, falling back to defaults...")
		targets, err = serverList.FindServer([]int{serverID})
		if err != nil {
			log.Printf("Error finding server: %v\n", err)
			return err
		}
	}

	latency, downspeed, upspeed, err = TestServer(targets[0])
	if err != nil {
		return err
	}

	if os.Getenv("DEBUG") != "" {
		log.Println("-------------------------------------")
		log.Println("Debug enabled, testing all servers...")
		for _, target := range serverList {
			TestServer(target)
		}
		log.Println("-------------------------------------")
	}

	return nil
}

func TestServer(target *speedtest.Server) (time.Duration, float64, float64, error) {
	var err error

	err = target.PingTest()
	if err != nil {
		log.Printf("Error running ping test: %v\n", err)
		return 0, 0, 0, err
	}
	err = target.DownloadTest(false)
	if err != nil {
		log.Printf("Error running download test: %v\n", err)
		return 0, 0, 0, err
	}
	err = target.UploadTest(false)
	if err != nil {
		log.Printf("Error runing upload test: %v\n", err)
		return 0, 0, 0, err
	}

	if target.DLSpeed > 0 && target.ULSpeed > 0 && target.DLSpeed < cutOff && target.ULSpeed < cutOff {
		log.Printf("Testing against server: %s - %s [%s] - DL=%f UL=%f Ping=%v\n",
			target.Name, target.Sponsor, target.ID,
			target.DLSpeed, target.ULSpeed, target.Latency,
		)
	} else {
		log.Printf("Testing against server: %s - %s [%s] - Failed to get valid results\n",
			target.Name, target.Sponsor, target.ID,
		)
		err = fmt.Errorf("invalid test results")
	}

	return target.Latency, target.DLSpeed, target.ULSpeed, err
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
