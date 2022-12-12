package main

import (
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	log "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/kr/pretty"
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
	listenAddress          = kingpin.Flag("web.listen-address", "Address on which to expose metrics.").Default(":9799").String()
	metricsEndpoint        = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	disableExporterMetrics = kingpin.Flag(
		"web.disable-exporter-metrics",
		"Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).",
	).Bool()
	dump1090Address = kingpin.Flag("dump1090.address",
		`Address of dump1090 service, e.g. http://localhost:80/dump1090/data/. Either dump1090.address or dump1090.files must be supplied`).
		String()
	dump1090Files = kingpin.Flag("dump1090.files",
		`Location of dump1090 JSON files (e.g. /run/dump1090/%s or /dev/shm/rbfeeder_%s). Either dump1090.address or dump1090.files must be supplied`).
		String()
	//compassPointStr = kingpin.Flag("compass.points", "Compass points.").Default("N,NE,E,SE,S,SW,W,NW").String()
	compassPointStr = kingpin.Flag("compass.points", "Compass points.").Default("000,045,090,135,180,225,270,315").String()
	compassPoints   []string

	logger log.Logger

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
			[]string{"with_position", "direction"},
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
			[]string{"direction"},
			nil),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.aircraftCount
	ch <- e.messageCount
	ch <- e.timestamp
	ch <- e.maxDistance
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

	ch <- prometheus.MustNewConstMetric(e.messageCount, prometheus.CounterValue, float64(aircraft.Messages))
	ch <- prometheus.MustNewConstMetric(e.timestamp, prometheus.CounterValue, aircraft.Now)

	// Now calculate distances from us, if possible.
	if receiver.Lat != 0 || receiver.Lon != 0 {
		start := time.Now()
		here := osgridref.LatLon{Lat: receiver.Lat, Lon: receiver.Lon}
		maxDistances := make([]float64, len(compassPoints))
		counts := make([]float64, len(compassPoints))
		withoutPosition := 0

		for _, a := range aircraft.Aircraft {
			if a.Lat != 0 || a.Lon != 0 {
				d := here.DistanceTo(osgridref.LatLon{Lat: a.Lat, Lon: a.Lon})
				// - Distance in naut miles:N fmt.Println(d / 1852)
				s := sector(len(compassPoints), int(here.InitialBearingTo(osgridref.LatLon{Lat: a.Lat, Lon: a.Lon})))
				counts[s]++
				if d > maxDistances[s] {
					maxDistances[s] = d
				}
			} else {
				withoutPosition++
			}
		}

		for i := range maxDistances {
			ch <- prometheus.MustNewConstMetric(e.maxDistance, prometheus.GaugeValue, maxDistances[i], compassPoints[i])
			ch <- prometheus.MustNewConstMetric(e.aircraftCount, prometheus.GaugeValue, counts[i], "true", compassPoints[i])
		}
		ch <- prometheus.MustNewConstMetric(e.aircraftCount, prometheus.GaugeValue, float64(withoutPosition), "false", "")

		level.Debug(logger).Log("calcs", havePosition, "time", time.Since(start))
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
		Track    float64       `json:"track,omitempty"`
		Speed    float64       `json:"speed,omitempty"`
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
	var (
		aircraft Aircraft
		receiver Receiver
	)

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

	if *dump1090Files != "" {
		path := fmt.Sprintf(*dump1090Files, file)
		r, err = os.Open(path)
	} else {
		var resp *http.Response
		resp, err = http.Get(*dump1090Address + "/" + file)
		if err == nil {
			r = resp.Body
		}
	}

	if err != nil {
		level.Warn(logger).Log("file", file, "error", err)
		return err
	}

	defer r.Close()

	decoder := json.NewDecoder(r)
	return decoder.Decode(obj)
}

func main() {
	kingpin.Version(version.Print("dump1090_exporter"))
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.Parse()

	logger = promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting dump1090_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	compassPoints = strings.Split(*compassPointStr, ",")
	level.Info(logger).Log("compassPoints", *compassPointStr)

	if user, err := user.Current(); err == nil && user.Uid == "0" {
		level.Warn(logger).Log("msg", "Exporter is running as root user. This exporter is designed to run as unprivileged user, root is not required.")
	}

	if (*dump1090Files == "") == (*dump1090Address == "") {
		stdlog.Fatal("Must supply exactly one of --dump1090.files or --dump1090.address")
	}

	exporter := NewExporter()
	var registry = prometheus.DefaultRegisterer
	var gatherer = prometheus.DefaultGatherer
	if *disableExporterMetrics {
		reg := prometheus.NewRegistry()
		registry = reg
		gatherer = reg
	}
	registry.MustRegister(exporter)
	registry.MustRegister(version.NewCollector("dump1090_exporter"))

	http.Handle(*metricsEndpoint, promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
<html>
			<head><title>Dump1090 Exporter</title></head>
			<body>
			<h1>Dump1090 Exporter</h1>
			<p><a href="` + *metricsEndpoint + `">Metrics</a></p>
			</body>
</html>
`))
	})

	stdlog.Fatal(http.ListenAndServe(*listenAddress, nil))
}
