package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/hfoxy/tapo/pkg/tapo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	// these variables are set at build time

	// version is the version of the exporter
	version = "dev"

	// commitHash is the git commit hash of the exporter
	commitHash = "n/a"

	// buildTimestamp is the build timestamp of the exporter
	buildTimestamp = "n/a"
)

// Plug is the configuration for a plug
type Plug struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	IP       string `yaml:"ip"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Room     string `yaml:"room,omitempty"`

	tapo *tapo.Tapo
}

// PlugConfig is the configuration for the exporter
type PlugConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Plugs    []Plug `yaml:"plugs"`
}

var plugCurrentPowerGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_current_power", Help: "Plug Current Power"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugSignalLevelGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_signal_level", Help: "Plug Signal Level"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugRssiGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_rssi", Help: "Plug RSSI"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_status", Help: "Plug Status"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugOverheatedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_overheated", Help: "Plug Overheated"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugTodayRuntimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_today_runtime", Help: "Plug Today Runtime"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugMonthRuntimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_month_runtime", Help: "Plug Month Runtime"}, []string{"id", "plug_name", "room", "plug_ip"})
var plugOnTimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_on_time", Help: "Plug On Time"}, []string{"id", "plug_name", "room", "plug_ip"})

var plugs map[string]*Plug

func main() {
	log.Printf("Starting tapo-prometheus-exporter %s (%s @ %s)", version, commitHash, buildTimestamp)

	ctx, ctxClose := context.WithCancel(context.Background())
	defer ctxClose()

	configPath, ok := os.LookupEnv("CONFIG_PATH")
	if !ok {
		configPath = "config.yaml"
	}

	err := refreshConfig(configPath)
	if err != nil {
		panic(err)
	}

	var w *fsnotify.Watcher
	w, err = fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	defer w.Close()

	go func() {
		for {
			select {
			case event, wok := <-w.Events:
				if !wok {
					return
				}

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Printf("config file updated")
					err = refreshConfig(configPath)
					if err != nil {
						log.Printf("unable to refresh config: %v", err)
					}

					continue
				}

				w.Add(event.Name) // re-add the file here
			case werr, wok := <-w.Errors:
				if !wok {
					return
				}

				log.Printf("unable to watch file: %v", werr)
			}
		}
	}()

	err = w.Add(configPath)
	if err != nil {
		log.Printf("unable to watch config file: %v", err)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				updatePlugs()
			case <-ctx.Done():
				ticker.Stop()
				os.Exit(0)
			}
		}
	}()

	updatePlugs()

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err = fmt.Fprintf(w, "OK")
		if err != nil {
			log.Printf("unable to write response: %v", err)
			return
		}
	}))
	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(":8080", logRequestHandler(http.DefaultServeMux))
	if err != nil {
		panic(err)
	}
}

func refreshConfig(configPath string) error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		panic(fmt.Errorf("unable to read config.yaml: %v", err))
	}

	var config PlugConfig
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal config.yaml: %v", err))
	}

	newPlugs := make(map[string]*Plug)
	newPlugsByName := make(map[string]*Plug)

	err = nil
	for _, plug := range config.Plugs {
		log.Printf("adding plug %s (%s)", plug.Name, plug.IP)

		if plug.Username == "" {
			plug.Username = config.Username
		}

		if plug.Password == "" {
			plug.Password = config.Password
		}

		if plug.Room == "" {
			plug.Room = "default"
		}

		if plug.ID == "" {
			err = errors.Join(err, fmt.Errorf("id must be provided for plug '%s'", plug.Name))
		}

		if plug.Name == "" {
			err = errors.Join(err, fmt.Errorf("name must be provided for plug '%s'", plug.ID))
		}

		if plug.IP == "" {
			err = errors.Join(err, fmt.Errorf("ip must be provided for plug %s", plug.Name))
		}

		if _, ok := newPlugs[plug.ID]; ok {
			err = errors.Join(err, fmt.Errorf("duplicate plug id '%s'", plug.ID))
		} else if _, ok := newPlugsByName[plug.Name]; ok {
			err = errors.Join(err, fmt.Errorf("duplicate plug name '%s'", plug.Name))
		}

		if plug.Username == "" || plug.Password == "" {
			err = errors.Join(err, fmt.Errorf("username and password must be provided for plug '%s'", plug.ID))
		} else if err == nil {
			tapoPlug, plugErr := tapo.NewTapo(plug.IP, plug.Username, plug.Password)
			if plugErr != nil {
				log.Printf("unable to create Tapo plug '%s': %v", plug.ID, plugErr)
			} else {
				plug.tapo = tapoPlug
				newPlugs[plug.ID] = &plug
				newPlugsByName[plug.Name] = &plug
				log.Printf("added plug '%s' (%s)", plug.Name, plug.IP)
			}
		}
	}

	if err != nil {
		return err
	}

	plugs = newPlugs
	return nil
}

func updatePlugs() {
	updated := 0
	for id, plug := range plugs {
		var plugErr error
		var plugIP = plug.IP
		var room = plug.Room
		var name = plug.Name

		dri, drie := plug.tapo.GetDeviceRunningInfo()
		if drie != nil {
			plugErr = errors.Join(plugErr, fmt.Errorf("unable to get device running info for plug %s: %v", name, drie))
		}

		/*di, die := plug.GetDeviceInfo()
		if die != nil {
			plugErr = errors.Join(plugErr, fmt.Errorf("unable to get device info for plug %s: %v", name, die))
		}*/

		eu, eue := plug.tapo.GetEnergyUsage()
		if eue != nil {
			plugErr = errors.Join(plugErr, fmt.Errorf("unable to get energy usage for plug %s: %v", name, eue))
		}

		if drie == nil && dri.Result.IP == "" {
			plugErr = errors.Join(plugErr, fmt.Errorf("unable to get device ip for plug %s: %v", name, dri))
		}

		if plugErr != nil {
			log.Printf("unable to get data for plug %s: %v", name, plugErr)
			continue
		}

		plugCurrentPowerGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(eu.Result.CurrentPower))
		plugSignalLevelGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(dri.Result.SignalLevel))
		plugRssiGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(dri.Result.Rssi))
		plugTodayRuntimeGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(eu.Result.TodayRuntime))
		plugMonthRuntimeGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(eu.Result.MonthRuntime))
		plugOnTimeGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(float64(dri.Result.OnTime))

		status := float64(0)
		if dri.Result.DeviceOn {
			status = 1
		}

		plugStatusGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(status)

		overheated := float64(0)
		if dri.Result.Overheated {
			overheated = 1
		}

		plugOverheatedGauge.With(prometheus.Labels{"id": id, "plug_name": name, "room": room, "plug_ip": plugIP}).Set(overheated)
		updated++
	}

	log.Printf("updated %d plugs", updated)
}

func logRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		uri := r.URL.String()
		method := r.Method
		log.Printf("%s %s", method, uri)
	}

	return http.HandlerFunc(fn)
}
