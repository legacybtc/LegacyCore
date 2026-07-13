package ai

import (
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type GPUInfo struct {
	Vendor             string   `json:"vendor"`
	Name               string   `json:"name"`
	VRAMMB             int      `json:"vram_mb"`
	CUDAAvailable      bool     `json:"cuda"`
	ROCmAvailable      bool     `json:"rocm"`
	VulkanAvailable    bool     `json:"vulkan"`
	RecommendedBackend string   `json:"recommended"`
	FallbackReason     string   `json:"fallback_reason"`
	AvailableBackends  []string `json:"available_backends"`
}

var (
	gpuCache     GPUInfo
	gpuCacheMu   sync.RWMutex
	gpuCachedAt  time.Time
	gpuCacheTTL  = 5 * time.Minute
)

func DetectGPU() GPUInfo {
	gpuCacheMu.RLock()
	cached := gpuCache
	cachedAt := gpuCachedAt
	gpuCacheMu.RUnlock()
	if !cachedAt.IsZero() && time.Since(cachedAt) < gpuCacheTTL {
		return cached
	}
	info := detectGPU()
	gpuCacheMu.Lock()
	gpuCache = info
	gpuCachedAt = time.Now()
	gpuCacheMu.Unlock()
	return info
}

func detectGPU() GPUInfo {
	info := GPUInfo{Vendor: "none", Name: "CPU only", RecommendedBackend: "mock", AvailableBackends: []string{"mock"}}

	info.CUDAAvailable = binaryExists("nvidia-smi")
	info.ROCmAvailable = binaryExists("rocm-smi")
	info.VulkanAvailable = binaryExists("vulkaninfo")

	if info.CUDAAvailable {
		name, vram := queryNVIDIA()
		if name != "" {
			info.Vendor = "nvidia"
			info.Name = name
			info.VRAMMB = vram
			info.AvailableBackends = append(info.AvailableBackends, "cuda")
			if runtime.GOOS != "windows" || hasDLL("nvcuda.dll") {
				info.RecommendedBackend = "llama-cpp-cuda"
			}
		}
	} else if info.ROCmAvailable {
		name, vram := queryROCm()
		if name != "" {
			info.Vendor = "amd"
			info.Name = name
			info.VRAMMB = vram
			info.AvailableBackends = append(info.AvailableBackends, "rocm")
			info.RecommendedBackend = "llama-cpp-rocm"
		}
	}

	if info.VulkanAvailable && info.Vendor == "none" {
		info.AvailableBackends = append(info.AvailableBackends, "vulkan")
		info.RecommendedBackend = "llama-cpp-vulkan"
	}

	if info.Vendor == "none" {
		info.FallbackReason = "No GPU detected"
	}

	return info
}

func queryNVIDIA() (string, int) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return "", 0
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return "", 0
	}
	name := strings.TrimSpace(parts[0])
	var vram int
	_, _ = parseVRAM(strings.TrimSpace(parts[1]), &vram)
	return name, vram
}

func queryROCm() (string, int) {
	out, err := exec.Command("rocm-smi", "--showproductname", "--showmeminfo", "vram", "--csv").Output()
	if err != nil {
		return "", 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", 0
	}
	parts := strings.Split(lines[1], ",")
	if len(parts) < 2 {
		return "", 0
	}
	name := strings.TrimSpace(parts[0])
	var vram int
	_, _ = parseVRAM(strings.TrimSpace(parts[1]), &vram)
	return name, vram
}

func parseVRAM(s string, dst *int) (int, error) {
	val := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
		} else {
			break
		}
	}
	*dst = val
	return val, nil
}

func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hasDLL(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
