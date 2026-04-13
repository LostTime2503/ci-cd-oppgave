package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dataDir = getEnv("DATA_DIR", "/data")

	jobFileCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobreader_files_total",
		Help: "Total number of data files in the data directory",
	})

	jobFileAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "jobreader_file_age_seconds",
		Help: "Age of each data file in seconds since last modification",
	}, []string{"filename"})

	jobFileSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "jobreader_file_size_bytes",
		Help: "Size of each data file in bytes",
	}, []string{"filename"})

	jobLastUpdate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobreader_last_update_timestamp",
		Help: "Unix timestamp of the most recently updated file",
	})

	jobStaleFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobreader_stale_files_total",
		Help: "Number of files not updated in the last 10 minutes (possible failed jobs)",
	})

	mu sync.RWMutex
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	prometheus.MustRegister(jobFileCount)
	prometheus.MustRegister(jobFileAge)
	prometheus.MustRegister(jobFileSize)
	prometheus.MustRegister(jobLastUpdate)
	prometheus.MustRegister(jobStaleFiles)
}

func updateMetrics() {
	for {
		mu.RLock()
		entries, err := os.ReadDir(dataDir)
		mu.RUnlock()
		if err != nil {
			log.Printf("Error reading data dir %s: %v", dataDir, err)
			time.Sleep(5 * time.Second)
			continue
		}

		count := 0
		stale := 0
		var latestMod time.Time

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			count++
			info, err := e.Info()
			if err != nil {
				continue
			}
			name := e.Name()
			age := time.Since(info.ModTime()).Seconds()
			jobFileAge.WithLabelValues(name).Set(age)
			jobFileSize.WithLabelValues(name).Set(float64(info.Size()))

			if info.ModTime().After(latestMod) {
				latestMod = info.ModTime()
			}
			if age > 600 {
				stale++
			}
		}

		jobFileCount.Set(float64(count))
		jobStaleFiles.Set(float64(stale))
		if !latestMod.IsZero() {
			jobLastUpdate.Set(float64(latestMod.Unix()))
		}

		time.Sleep(10 * time.Second)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		http.Error(w, "Could not read data directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Job Data Reader</title>
<meta http-equiv="refresh" content="30">
<style>
body { font-family: monospace; margin: 2em; background: #1a1a2e; color: #e0e0e0; }
h1 { color: #00d4ff; }
h2 { color: #aaa; }
.file { background: #16213e; padding: 1em; margin: 1em 0; border-radius: 8px; border-left: 4px solid #00d4ff; }
.file.stale { border-left-color: #ff4444; }
.filename { color: #00d4ff; font-weight: bold; font-size: 1.1em; }
.meta { color: #888; font-size: 0.9em; margin: 0.3em 0; }
.stale-warning { color: #ff4444; font-weight: bold; }
pre { white-space: pre-wrap; word-wrap: break-word; }
.legend { background: #16213e; padding: 1em; border-radius: 8px; margin-bottom: 1em; }
.legend span { margin-right: 2em; }
</style></head><body>
<h1>Jobbdata fra skedulerte kjoeringer</h1>
<h2>Simulerer IBM IWS/TWS jobbskedulering via Kubernetes CronJobs og Jobs</h2>
<div class="legend">
  <span style="border-left: 4px solid #00d4ff; padding-left: 0.5em;">Oppdatert</span>
  <span style="border-left: 4px solid #ff4444; padding-left: 0.5em;">Utgaatt (&gt;10 min)</span>
</div>
`)

	if len(entries) == 0 {
		fmt.Fprint(w, `<p><em>Ingen datafiler funnet ennaa. Vent paa at jobbene kjoerer...</em></p>`)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dataDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info, _ := e.Info()

		staleClass := ""
		staleLabel := ""
		if info != nil && time.Since(info.ModTime()) > 10*time.Minute {
			staleClass = " stale"
			staleLabel = ` <span class="stale-warning">[UTGAATT - jobb feilet?]</span>`
		}

		fmt.Fprintf(w, `<div class="file%s">`, staleClass)
		fmt.Fprintf(w, `<div class="filename">%s%s</div>`, html.EscapeString(e.Name()), staleLabel)
		if info != nil {
			age := time.Since(info.ModTime()).Truncate(time.Second)
			fmt.Fprintf(w, `<div class="meta">Sist oppdatert: %s | Alder: %s | Størrelse: %d bytes</div>`,
				info.ModTime().Format("2006-01-02 15:04:05"),
				age.String(),
				info.Size())
		}
		fmt.Fprintf(w, `<pre>%s</pre>`, html.EscapeString(string(content)))
		fmt.Fprint(w, `</div>`)
	}

	fmt.Fprint(w, `</body></html>`)
}

func main() {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Warning: could not create data dir %s: %v", dataDir, err)
	}

	go updateMetrics()

	http.HandleFunc("/", homeHandler)
	http.Handle("/metrics", promhttp.Handler())

	fmt.Printf("Job Reader starting on :8080, reading from %s\n", dataDir)
	log.Fatal(http.ListenAndServe(":8080", nil))
}