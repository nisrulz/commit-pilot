package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MinContextFloor = 256

type ModelInfo struct {
	Key              string
	MaxContextLength int64
}

func DetectContextWindow(apiBase string) int {
	totalRAM, err := GetSystemRAM()
	if err != nil {
		return 0
	}
	const reservedRAM int64 = 5 << 30
	availableRAM := totalRAM - reservedRAM
	if availableRAM <= 0 {
		return MinContextFloor
	}

	mi, err := QueryModelInfo(apiBase)
	if err != nil {
		return 0
	}

	maxCL := int(mi.MaxContextLength)

	if _, err := exec.LookPath("lms"); err == nil {
		return SearchMaxContext(availableRAM, mi)
	}

	return maxCL
}

func QueryModelInfo(apiBase string) (*ModelInfo, error) {
	if err := ValidateProviderURL(apiBase); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/models", strings.TrimRight(apiBase, "/"))
	client := newProviderHTTPClient(10 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var payload struct {
		Models []struct {
			Key              string `json:"key"`
			MaxContextLength int64  `json:"max_context_length"`
		} `json:"models"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxResponseSize)).Decode(&payload); err != nil {
		return nil, err
	}

	for _, m := range payload.Models {
		if m.MaxContextLength > 0 {
			return &ModelInfo{Key: m.Key, MaxContextLength: m.MaxContextLength}, nil
		}
	}
	return nil, fmt.Errorf("no LLM model found")
}

func SearchMaxContext(availableRAM int64, mi *ModelInfo) int {
	maxCL := int(mi.MaxContextLength)

	needed := EstimateMemory(mi.Key, maxCL)
	if needed < 0 {
		return maxCL
	}
	if needed <= availableRAM {
		return maxCL
	}

	lo := MinContextFloor
	if lo > maxCL {
		lo = maxCL
	}
	hi := maxCL
	best := lo
	for lo <= hi {
		mid := (lo + hi) / 2
		needed := EstimateMemory(mi.Key, mid)
		if needed < 0 {
			return best
		}
		if needed <= availableRAM {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func EstimateMemory(modelKey string, cl int) int64 {
	cmd := exec.Command("lms", "load", "--estimate-only", modelKey,
		"--context-length", strconv.Itoa(cl))
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	return ParseEstimatedMemory(string(out))
}

func ParseEstimatedMemory(output string) int64 {
	re := regexp.MustCompile(`Estimated Total Memory:\s+([\d,.]+)\s*(MiB|GiB)`)
	m := re.FindStringSubmatch(output)
	if len(m) < 3 {
		return -1
	}

	val := parseFloat(strings.ReplaceAll(m[1], ",", ""))
	unit := m[2]

	switch unit {
	case "GiB":
		return int64(val * (1 << 30))
	case "MiB":
		return int64(val * (1 << 20))
	default:
		return -1
	}
}

func GetSystemRAM() (int64, error) {
	if _, err := exec.LookPath("sysctl"); err == nil {
		cmd := exec.Command("sysctl", "-n", "hw.memsize")
		out, err := cmd.Output()
		if err == nil {
			v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			if err == nil {
				return v, nil
			}
		}
	}

	if _, err := os.Stat("/proc/meminfo"); err == nil {
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			re := regexp.MustCompile(`MemTotal:\s*(\d+) kB`)
			m := re.FindStringSubmatch(string(data))
			if len(m) > 1 {
				v, err := strconv.ParseInt(m[1], 10, 64)
				if err == nil {
					return v * 1024, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("unable to detect system RAM")
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
