package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const minContextFloor = 256

type modelInfo struct {
	key              string
	maxContextLength int64
}

func detectContextWindow(apiBase string) int {
	totalRAM, err := getSystemRAM()
	if err != nil {
		return 0
	}
	const reservedRAM int64 = 5 << 30
	availableRAM := totalRAM - reservedRAM
	if availableRAM <= 0 {
		return minContextFloor
	}

	mi, err := queryModelInfo(apiBase)
	if err != nil {
		return 0
	}

	maxCL := int(mi.maxContextLength)

	if _, err := exec.LookPath("lms"); err == nil {
		return searchMaxContext(availableRAM, mi)
	}

	return maxCL
}

func queryModelInfo(apiBase string) (*modelInfo, error) {
	url := fmt.Sprintf("%s/api/v1/models", strings.TrimRight(apiBase, "/"))
	resp, err := http.Get(url)
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

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	for _, m := range payload.Models {
		if m.MaxContextLength > 0 {
			return &modelInfo{key: m.Key, maxContextLength: m.MaxContextLength}, nil
		}
	}
	return nil, fmt.Errorf("no LLM model found")
}

func searchMaxContext(availableRAM int64, mi *modelInfo) int {
	maxCL := int(mi.maxContextLength)

	needed := estimateMemory(mi.key, maxCL)
	if needed < 0 {
		return maxCL
	}
	if needed <= availableRAM {
		return maxCL
	}

	lo := minContextFloor
	if lo > maxCL {
		lo = maxCL
	}
	hi := maxCL
	best := lo
	for lo <= hi {
		mid := (lo + hi) / 2
		needed := estimateMemory(mi.key, mid)
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

func estimateMemory(modelKey string, cl int) int64 {
	cmd := exec.Command("lms", "load", "--estimate-only", modelKey,
		"--context-length", strconv.Itoa(cl))
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	return parseEstimatedMemory(string(out))
}

func parseEstimatedMemory(output string) int64 {
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

func getSystemRAM() (int64, error) {
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
