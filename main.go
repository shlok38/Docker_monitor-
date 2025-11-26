package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ContainerStats holds the statistics for a container
type ContainerStats struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	BlockRead     uint64  `json:"block_read"`
	BlockWrite    uint64  `json:"block_write"`
}

// Monitor represents the Docker monitoring service
type Monitor struct {
	client *client.Client
	ctx    context.Context
}

// NewMonitor creates a new Docker monitor
func NewMonitor() (*Monitor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Monitor{
		client: cli,
		ctx:    context.Background(),
	}, nil
}

// GetContainerStats retrieves statistics for all running containers
func (m *Monitor) GetContainerStats() ([]ContainerStats, error) {
	containers, err := m.client.ContainerList(m.ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var stats []ContainerStats
	for _, ctr := range containers {
		// Get container name
		containerName := ctr.ID[:12] // Use ID as fallback
		if len(ctr.Names) > 0 {
			containerName = ctr.Names[0]
		}
		
		stat, err := m.getContainerStat(ctr.ID, containerName)
		if err != nil {
			log.Printf("Warning: failed to get stats for container %s: %v", ctr.ID[:12], err)
			continue
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// getContainerStat retrieves statistics for a single container
func (m *Monitor) getContainerStat(containerID, containerName string) (ContainerStats, error) {
	stats, err := m.client.ContainerStats(m.ctx, containerID, false)
	if err != nil {
		return ContainerStats{}, err
	}
	defer stats.Body.Close()

	var v container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return ContainerStats{}, err
	}

	// Calculate CPU percentage
	cpuPercent := calculateCPUPercent(&v)

	// Calculate memory percentage
	var memPercent float64
	if v.MemoryStats.Limit > 0 {
		memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
	}

	// Calculate network stats
	var rxBytes, txBytes uint64
	for _, network := range v.Networks {
		rxBytes += network.RxBytes
		txBytes += network.TxBytes
	}

	// Calculate block IO stats
	var blockRead, blockWrite uint64
	for _, bio := range v.BlkioStats.IoServiceBytesRecursive {
		if bio.Op == "Read" {
			blockRead += bio.Value
		} else if bio.Op == "Write" {
			blockWrite += bio.Value
		}
	}

	// Safely truncate container ID
	displayID := containerID
	if len(containerID) > 12 {
		displayID = containerID[:12]
	}

	return ContainerStats{
		ID:            displayID,
		Name:          containerName,
		CPUPercent:    cpuPercent,
		MemoryUsage:   v.MemoryStats.Usage,
		MemoryLimit:   v.MemoryStats.Limit,
		MemoryPercent: memPercent,
		NetworkRx:     rxBytes,
		NetworkTx:     txBytes,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
	}, nil
}

// calculateCPUPercent calculates the CPU usage percentage
func calculateCPUPercent(v *container.StatsResponse) float64 {
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

// Close closes the Docker client connection
func (m *Monitor) Close() error {
	return m.client.Close()
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// printStats prints container statistics to console
func printStats(stats []ContainerStats) {
	fmt.Print("\033[2J\033[H") // Clear screen
	fmt.Println("Docker Container Monitor")
	fmt.Println("========================")
	fmt.Printf("Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	if len(stats) == 0 {
		fmt.Println("No running containers found.")
		return
	}

	fmt.Printf("%-15s %-30s %10s %20s %15s %15s %15s %15s\n",
		"CONTAINER ID", "NAME", "CPU %", "MEMORY", "MEM %", "NET I/O", "BLOCK I/O", "")
	fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------")

	for _, stat := range stats {
		memUsage := fmt.Sprintf("%s / %s", formatBytes(stat.MemoryUsage), formatBytes(stat.MemoryLimit))
		netIO := fmt.Sprintf("%s / %s", formatBytes(stat.NetworkRx), formatBytes(stat.NetworkTx))
		blockIO := fmt.Sprintf("%s / %s", formatBytes(stat.BlockRead), formatBytes(stat.BlockWrite))

		fmt.Printf("%-15s %-30s %9.2f%% %20s %14.2f%% %15s %15s\n",
			stat.ID,
			stat.Name,
			stat.CPUPercent,
			memUsage,
			stat.MemoryPercent,
			netIO,
			blockIO,
		)
	}
}

// startCLI starts the CLI monitoring mode
func startCLI(monitor *Monitor, interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Print initial stats
	stats, err := monitor.GetContainerStats()
	if err != nil {
		log.Printf("Error getting container stats: %v", err)
	} else {
		printStats(stats)
	}

	for {
		select {
		case <-ticker.C:
			stats, err := monitor.GetContainerStats()
			if err != nil {
				log.Printf("Error getting container stats: %v", err)
				continue
			}
			printStats(stats)
		case <-sigChan:
			fmt.Println("\nShutting down...")
			return
		}
	}
}

// startAPI starts the REST API server
func startAPI(monitor *Monitor, port int) {
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := monitor.GetContainerStats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, dashboardHTML)
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting API server on http://localhost%s", addr)
	log.Printf("Dashboard available at http://localhost%s", addr)
	log.Printf("API endpoint: http://localhost%s/api/stats", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}

var dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docker Container Monitor</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        h1 {
            color: white;
            text-align: center;
            margin-bottom: 10px;
            font-size: 2.5em;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .subtitle {
            color: rgba(255,255,255,0.9);
            text-align: center;
            margin-bottom: 30px;
            font-size: 1.1em;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(600px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 20px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .card:hover {
            transform: translateY(-5px);
            box-shadow: 0 15px 40px rgba(0,0,0,0.3);
        }
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
            padding-bottom: 15px;
            border-bottom: 2px solid #f0f0f0;
        }
        .container-name {
            font-size: 1.3em;
            font-weight: bold;
            color: #667eea;
            word-break: break-all;
        }
        .container-id {
            font-size: 0.9em;
            color: #999;
            font-family: 'Courier New', monospace;
        }
        .stats-row {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 15px;
            margin-bottom: 10px;
        }
        .stat {
            background: #f8f9fa;
            padding: 12px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }
        .stat-label {
            font-size: 0.85em;
            color: #666;
            margin-bottom: 5px;
            text-transform: uppercase;
            font-weight: 600;
        }
        .stat-value {
            font-size: 1.2em;
            font-weight: bold;
            color: #333;
        }
        .progress-bar {
            width: 100%;
            height: 8px;
            background: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin-top: 5px;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea 0%, #764ba2 100%);
            border-radius: 4px;
            transition: width 0.3s ease;
        }
        .error {
            background: #fee;
            color: #c33;
            padding: 20px;
            border-radius: 12px;
            text-align: center;
            margin: 20px 0;
            border: 2px solid #fcc;
        }
        .loading {
            text-align: center;
            padding: 40px;
            color: white;
            font-size: 1.5em;
        }
        .timestamp {
            color: rgba(255,255,255,0.8);
            text-align: center;
            margin-bottom: 20px;
            font-size: 0.9em;
        }
        .no-containers {
            background: white;
            padding: 40px;
            border-radius: 12px;
            text-align: center;
            color: #666;
            font-size: 1.2em;
        }
        @media (max-width: 768px) {
            .stats-grid {
                grid-template-columns: 1fr;
            }
            .stats-row {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🐳 Docker Container Monitor</h1>
        <div class="subtitle">Real-time Container Statistics Dashboard</div>
        <div class="timestamp" id="timestamp"></div>
        <div id="stats-container">
            <div class="loading">Loading container statistics...</div>
        </div>
    </div>

    <script>
        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function updateStats() {
            fetch('/api/stats')
                .then(response => response.json())
                .then(data => {
                    const container = document.getElementById('stats-container');
                    const timestamp = document.getElementById('timestamp');
                    
                    timestamp.textContent = 'Last updated: ' + new Date().toLocaleString();

                    if (!data || data.length === 0) {
                        container.innerHTML = '<div class="no-containers">No running containers found</div>';
                        return;
                    }

                    let html = '<div class="stats-grid">';
                    data.forEach(stat => {
                        html += '<div class="card">';
                        html += '<div class="card-header">';
                        html += '<div class="container-name">' + stat.name + '</div>';
                        html += '<div class="container-id">' + stat.id + '</div>';
                        html += '</div>';
                        html += '<div class="stats-row">';
                        html += '<div class="stat">';
                        html += '<div class="stat-label">CPU Usage</div>';
                        html += '<div class="stat-value">' + stat.cpu_percent.toFixed(2) + '%</div>';
                        html += '<div class="progress-bar">';
                        html += '<div class="progress-fill" style="width: ' + Math.min(stat.cpu_percent, 100) + '%"></div>';
                        html += '</div></div>';
                        html += '<div class="stat">';
                        html += '<div class="stat-label">Memory Usage</div>';
                        html += '<div class="stat-value">' + stat.memory_percent.toFixed(2) + '%</div>';
                        html += '<div class="progress-bar">';
                        html += '<div class="progress-fill" style="width: ' + stat.memory_percent + '%"></div>';
                        html += '</div>';
                        html += '<div style="font-size: 0.8em; color: #666; margin-top: 5px;">';
                        html += formatBytes(stat.memory_usage) + ' / ' + formatBytes(stat.memory_limit);
                        html += '</div></div></div>';
                        html += '<div class="stats-row">';
                        html += '<div class="stat">';
                        html += '<div class="stat-label">Network I/O</div>';
                        html += '<div class="stat-value" style="font-size: 0.9em;">';
                        html += '&#8595; ' + formatBytes(stat.network_rx) + '<br>';
                        html += '&#8593; ' + formatBytes(stat.network_tx);
                        html += '</div></div>';
                        html += '<div class="stat">';
                        html += '<div class="stat-label">Block I/O</div>';
                        html += '<div class="stat-value" style="font-size: 0.9em;">';
                        html += 'Read: ' + formatBytes(stat.block_read) + '<br>';
                        html += 'Write: ' + formatBytes(stat.block_write);
                        html += '</div></div></div></div>';
                    });
                    html += '</div>';

                    container.innerHTML = html;
                })
                .catch(error => {
                    console.error('Error fetching stats:', error);
                    document.getElementById('stats-container').innerHTML = 
                        '<div class="error">Error loading container statistics. Please ensure Docker is running and accessible.</div>';
                });
        }

        // Update stats immediately and then every 2 seconds
        updateStats();
        setInterval(updateStats, 2000);
    </script>
</body>
</html>
`

func main() {
	var (
		apiMode  = flag.Bool("api", false, "Start in API mode with web dashboard")
		port     = flag.Int("port", 8080, "Port for API server (use with -api)")
		interval = flag.Int("interval", 2, "Update interval in seconds for CLI mode")
	)
	flag.Parse()

	monitor, err := NewMonitor()
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Close()

	if *apiMode {
		startAPI(monitor, *port)
	} else {
		startCLI(monitor, *interval)
	}
}
