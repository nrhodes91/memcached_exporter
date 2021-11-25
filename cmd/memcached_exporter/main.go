// Copyright 2019 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/prometheus/memcached_exporter/pkg/exporter"
)

func main() {
	var (
		address       = kingpin.Flag("memcached.address", "Memcached server address.").Default("localhost:11211").String()
		timeout       = kingpin.Flag("memcached.timeout", "memcached connect timeout.").Default("1s").Duration()
		pidFile       = kingpin.Flag("memcached.pid-file", "Optional path to a file containing the memcached PID for additional metrics.").Default("").String()
		enableTls     = kingpin.Flag("memcached.tls.enable", "Enable TLS connections to memcached").Bool()
		certfile      = kingpin.Flag("memcached.tls.certfile", "Client certificate file.").Default("").String()
		keyfile       = kingpin.Flag("memcached.tls.keyfile", "Client private key file.").Default("").String()
		cafile        = kingpin.Flag("memcached.tls.cafile", "Client root CA file.").Default("").String()
		skipVerify    = kingpin.Flag("memcached.tls.skipverify", "Skip server certificate verification").Bool()
		serverName    = kingpin.Flag("memcached.tls.servername", "Memcached TLS certificate servername").Default("").String()
		webConfig     = webflag.AddFlags(kingpin.CommandLine)
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9150").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Version(version.Print("memcached_exporter"))
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting memcached_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	if *serverName == "" {
		*serverName = strings.Split(*address, ":")[0]
	}
	var (
		tlsConfig *tls.Config
		err       error
	)
	if *enableTls {
		tlsConfig, err = promconfig.NewTLSConfig(&promconfig.TLSConfig{
			CertFile:           *certfile,
			KeyFile:            *keyfile,
			CAFile:             *cafile,
			ServerName:         *serverName,
			InsecureSkipVerify: *skipVerify,
		})
		if err != nil {
			level.Error(logger).Log("msg", "Failed to create TLS config", "err", err)
			os.Exit(1)
		}

	}

	prometheus.MustRegister(version.NewCollector("memcached_exporter"))
	prometheus.MustRegister(exporter.New(*address, *timeout, logger, tlsConfig))

	if *pidFile != "" {
		procExporter := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{
			PidFn:     prometheus.NewPidFileFn(*pidFile),
			Namespace: exporter.Namespace,
		})
		prometheus.MustRegister(procExporter)
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Memcached Exporter</title></head>
             <body>
             <h1>Memcached Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	srv := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(srv, *webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error running HTTP server", "err", err)
		os.Exit(1)
	}
}
