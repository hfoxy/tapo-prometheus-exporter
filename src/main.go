package main

import (
	"context"
	"errors"
	"fmt"
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

type PlugConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Plugs    []struct {
		Name     string `yaml:"name"`
		IP       string `yaml:"ip"`
		Username string `yaml:"username,omitempty"`
		Password string `yaml:"password,omitempty"`
	} `yaml:"plugs"`
}

var plugCurrentPowerGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_current_power", Help: "Plug Current Power"}, []string{"plug_name", "plug_ip"})
var plugSignalLevelGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_signal_level", Help: "Plug Signal Level"}, []string{"plug_name", "plug_ip"})
var plugRssiGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_rssi", Help: "Plug RSSI"}, []string{"plug_name", "plug_ip"})
var plugStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_status", Help: "Plug Status"}, []string{"plug_name", "plug_ip"})
var plugOverheatedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_overheated", Help: "Plug Overheated"}, []string{"plug_name", "plug_ip"})
var plugTodayRuntimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_today_runtime", Help: "Plug Today Runtime"}, []string{"plug_name", "plug_ip"})
var plugMonthRuntimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_month_runtime", Help: "Plug Month Runtime"}, []string{"plug_name", "plug_ip"})
var plugOnTimeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "plug_on_time", Help: "Plug On Time"}, []string{"plug_name", "plug_ip"})

func main() {
	log.Printf("Starting tapo-prometheus-exporter v%s (%s @ %s)", Version, CommitHash, BuildTimestamp)

	ctx, ctxClose := context.WithCancel(context.Background())
	defer ctxClose()

	file, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(fmt.Errorf("unable to read config.yaml: %v", err))
	}

	var config PlugConfig
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal config.yaml: %v", err))
	}

	plugs := make(map[string]*tapo.Tapo)

	err = nil
	for _, plug := range config.Plugs {
		if plug.Username == "" {
			plug.Username = config.Username
		}

		if plug.Password == "" {
			plug.Password = config.Password
		}

		if plug.IP == "" {
			err = errors.Join(err, fmt.Errorf("ip must be provided for plug %s", plug.Name))
		}

		if _, ok := plugs[plug.Name]; ok {
			err = errors.Join(err, fmt.Errorf("duplicate plug name %s", plug.Name))
		}

		if plug.Username == "" || plug.Password == "" {
			err = errors.Join(err, fmt.Errorf("username and password must be provided for plug %s", plug.Name))
		} else {
			tapoPlug, plugErr := tapo.NewTapo(plug.IP, plug.Username, plug.Password)
			if plugErr != nil {
				err = errors.Join(err, fmt.Errorf("unable to create Tapo plug %s: %v", plug.Name, plugErr))
			} else {
				plugs[plug.Name] = tapoPlug
			}
		}
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				for name, plug := range plugs {
					var plugErr error

					dri, drie := plug.GetDeviceRunningInfo()
					if drie != nil {
						plugErr = errors.Join(plugErr, fmt.Errorf("unable to get device running info for plug %s: %v", name, drie))
					}

					/*di, die := plug.GetDeviceInfo()
					if die != nil {
						plugErr = errors.Join(plugErr, fmt.Errorf("unable to get device info for plug %s: %v", name, die))
					}*/

					eu, eue := plug.GetEnergyUsage()
					if eue != nil {
						plugErr = errors.Join(plugErr, fmt.Errorf("unable to get energy usage for plug %s: %v", name, eue))
					}

					plugCurrentPowerGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(eu.Result.CurrentPower))
					plugSignalLevelGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(dri.Result.SignalLevel))
					plugRssiGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(dri.Result.Rssi))
					plugTodayRuntimeGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(eu.Result.TodayRuntime))
					plugMonthRuntimeGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(eu.Result.MonthRuntime))
					plugOnTimeGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(float64(dri.Result.OnTime))

					status := float64(0)
					if dri.Result.DeviceOn {
						status = 1
					}

					plugStatusGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(status)

					overheated := float64(0)
					if dri.Result.Overheated {
						overheated = 1
					}

					plugOverheatedGauge.With(prometheus.Labels{"plug_name": name, "plug_ip": dri.Result.IP}).Set(overheated)
				}
			case <-ctx.Done():
				ticker.Stop()
				os.Exit(0)
			}
		}
	}()

	plugCurrentPowerGauge.With(prometheus.Labels{"plug_name": "plug1", "plug_ip": ""}).Set(1)

	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(":8080", logRequestHandler(http.DefaultServeMux))
	if err != nil {
		panic(err)
	}
}

func logRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

		// call the original http.Handler we're wrapping
		h.ServeHTTP(w, r)

		// gather information about request and log it
		uri := r.URL.String()
		method := r.Method

		// ... more information
		log.Printf("%s %s", method, uri)
	}

	// http.HandlerFunc wraps a function so that it
	// implements http.Handler interface
	return http.HandlerFunc(fn)
}
