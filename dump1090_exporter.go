package main

import (
	"encoding/json"
	"github.com/kr/pretty"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"os/user"
	"sync"
	"time"

	log "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/paulcager/osgridref"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "dump1090"
)

var (
	listenAddress   = kingpin.Flag("telemetry.address", "Address on which to expose metrics.").Default(":9799").String()
	metricsEndpoint = kingpin.Flag("telemetry.endpoint", "Path under which to expose metrics.").Default("/metrics").String()
	dump1090Address = kingpin.Flag("dump1090.address",
		`Address of dump1090 service, e.g. http://localhost:80/dump1090. Either dump1090.address or dump1090.directory must be supplied`).
		String()
	dump1090Directory = kingpin.Flag("dump1090.directory",
		`Directory containing dump1090 JSON files (e.g. /run/dump1090). Either dump1090.address or dump1090.directory must be supplied`).
		String()

	logger     log.Logger
	myLocation osgridref.LatLon

	_ = pretty.Print
)

type Exporter struct {
	mutex sync.Mutex

	aircraftCount *prometheus.Desc
	messageCount  *prometheus.Desc
	timestamp     *prometheus.Desc
	maxDistance   *prometheus.Desc
}

func NewExporter() *Exporter {
	return &Exporter{
		aircraftCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "aircraft", "count"),
			"Number of aircraft in view",
			[]string{"with_position"},
			nil),
		messageCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "aircraft", "messages"),
			"Number of ADSB messages received",
			nil,
			nil),
		timestamp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "aircraft", "timestamp"),
			"Timestamp of last message",
			nil,
			nil),
		maxDistance: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "aircraft", "max_distance"),
			"Maximum distance (meters)",
			nil,
			nil),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.aircraftCount
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	err := e.collect(ch)
	if err != nil {
		level.Error(logger).Log("fetch_error", err)
	}
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	aircraft, receiver, err := fetchFlights()
	if err != nil {
		return err
	}

	havePosition := 0
	for _, a := range aircraft.Aircraft {
		if a.Lat != 0 || a.Lon != 0 {
			// TODO - but what about aircraft at (0,0)??
			havePosition++
		}
	}

	ch <- prometheus.MustNewConstMetric(e.aircraftCount, prometheus.GaugeValue, float64(havePosition), "true")
	ch <- prometheus.MustNewConstMetric(e.aircraftCount, prometheus.GaugeValue, float64(len(aircraft.Aircraft)-havePosition), "false")
	ch <- prometheus.MustNewConstMetric(e.messageCount, prometheus.CounterValue, float64(aircraft.Messages))
	ch <- prometheus.MustNewConstMetric(e.timestamp, prometheus.CounterValue, float64(aircraft.Now))

	// Now calculate distances from us, if possible.
	if receiver.Lat != 0 || receiver.Lon != 0 {
		start := time.Now()
		here := osgridref.LatLon{Lat: receiver.Lat, Lon: receiver.Lon}
		maxDistance := 0.0

		for _, a := range aircraft.Aircraft {
			if a.Lat != 0 || a.Lon != 0 {
				d := here.DistanceTo(osgridref.LatLon{Lat: a.Lat, Lon: a.Lon})
				// - Distance in naut miles:N fmt.Println(d / 1852)
				if d > maxDistance {
					maxDistance = d
				}
			}
		}

		level.Debug(logger).Log("calcs", havePosition, "time", time.Since(start))

		ch <- prometheus.MustNewConstMetric(e.maxDistance, prometheus.GaugeValue, maxDistance)
	}

	return nil
}

type Aircraft struct {
	Now      float64 `json:"now"`
	Messages int     `json:"messages"`
	Aircraft []struct {
		Hex      string        `json:"hex"`
		Squawk   string        `json:"squawk,omitempty"`
		Flight   string        `json:"flight,omitempty"`
		Lat      float64       `json:"lat,omitempty"`
		Lon      float64       `json:"lon,omitempty"`
		Nucp     int           `json:"nucp,omitempty"`
		SeenPos  float64       `json:"seen_pos,omitempty"`
		Altitude int           `json:"altitude,omitempty"`
		VertRate int           `json:"vert_rate,omitempty"`
		Track    int           `json:"track,omitempty"`
		Speed    int           `json:"speed,omitempty"`
		Category string        `json:"category,omitempty"`
		MLAT     []string      `json:"mlat"`
		TISB     []interface{} `json:"tisb"`
		Messages int           `json:"messages"`
		Seen     float64       `json:"seen"`
		RSSI     float64       `json:"rssi"`
	} `json:"aircraft"`
}

type Receiver struct {
	Version string  `json:"version"`
	Refresh int     `json:"refresh"`
	History int     `json:"history"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

func fetchFlights() (Aircraft, Receiver, error) {
	var aircraft Aircraft
	var receiver Receiver
	err := get("aircraft.json", &aircraft)
	if err == nil {
		err = get("receiver.json", &receiver)
	}

	return aircraft, receiver, err
}

func get(file string, obj interface{}) error {
	var (
		r   io.ReadCloser
		err error
	)

	if *dump1090Directory != "" {
		r, err = os.Open(*dump1090Directory + "/" + file)
	} else {
		var resp *http.Response
		resp, err = http.Get(*dump1090Address)
		if err == nil {
			r = resp.Body
		}
	}

	if err != nil {
		return err
	}

	defer r.Close()

	decoder := json.NewDecoder(r)
	return decoder.Decode(obj)
}

func main() {
	kingpin.Version(version.Print("dump1090_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.Parse()

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("node_exporter"))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger = promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting node_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	if user, err := user.Current(); err == nil && user.Uid == "0" {
		level.Warn(logger).Log("msg", "Exporter is running as root user. This exporter is designed to run as unprivileged user, root is not required.")
	}

	if (*dump1090Directory == "") == (*dump1090Address == "") {
		stdlog.Fatal("Must supply exactly one of --dump1090.directory or --dump1090.address")
	}

	exporter := NewExporter()
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("dump1090_exporter"))

	http.Handle(*metricsEndpoint, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="` + *metricsEndpoint + `">Metrics</a></p>
			</body>
</html>
`))
	})

	stdlog.Fatal(http.ListenAndServe(*listenAddress, nil))
}
