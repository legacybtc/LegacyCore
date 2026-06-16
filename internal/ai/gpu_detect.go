package ai

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type GPUInfo struct {
	Vendor           string   `json:"vendor"`
	Name             string   `json:"name"`
	VRAMMB           int      `json:"vram_mb"`
	CUDAAvailable    bool     `json:"cuda_available"`
	ROCmAvailable    bool     `json:"rocm_available"`
	VulkanAvailable  bool     `json:"vulkan_available"`
	AvailableBackends []string `json:"available_backends"`
	RecommendedBackend string  `json:"recommended_backend"`
	FallbackReason   string   `json:"fallback_reason,omitempty"`
}

func DetectGPU() GPUInfo {
	info := GPUInfo{}
	info.detect()
	info.recommend()
	return info
}

func (g *GPUInfo) detect() {
	if runtime.GOOS == "windows" {
		g.detectNVIDIAWindows()
		g.detectVulkan()
	} else {
		g.detectNVIDIALinux()
		g.detectAMDROCm()
		g.detectVulkan()
	}
	g.AvailableBackends = []string{"cpu"}
	if g.CUDAAvailable {
		g.AvailableBackends = append(g.AvailableBackends, "cuda")
	}
	if g.ROCmAvailable {
		g.AvailableBackends = append(g.AvailableBackends, "rocm")
	}
	if g.VulkanAvailable {
		g.AvailableBackends = append(g.AvailableBackends, "vulkan")
	}
}

func (g *GPUInfo) recommend() {
	switch {
	case g.CUDAAvailable:
		g.RecommendedBackend = "cuda"
	case g.ROCmAvailable:
		g.RecommendedBackend = "rocm"
	case g.VulkanAvailable:
		g.RecommendedBackend = "vulkan"
	default:
		g.RecommendedBackend = "cpu"
		g.FallbackReason = "no GPU detected; CPU-only inference"
	}
}

func (g *GPUInfo) detectNVIDIAWindows() {
	// nvidia-smi
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(fields) >= 2 {
		g.Vendor = "NVIDIA"
		g.Name = strings.TrimSpace(fields[0])
		g.VRAMMB, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
		g.CUDAAvailable = true
	}
}

func (g *GPUInfo) detectNVIDIALinux() {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(fields) >= 2 {
		g.Vendor = "NVIDIA"
		g.Name = strings.TrimSpace(fields[0])
		g.VRAMMB, _ = strconv.Atoi(strings.TrimSpace(fields[1]))
		g.CUDAAvailable = true
	}
}

func (g *GPUInfo) detectAMDROCm() {
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		g.ROCmAvailable = true
		out, err := exec.Command("rocm-smi", "--showproductname", "--csv").Output()
		if err == nil && g.Vendor == "" {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 1 {
				line := strings.TrimSpace(lines[1])
				if strings.Contains(line, ",") {
					g.Vendor = "AMD"
					g.Name = strings.TrimSpace(strings.SplitN(line, ",", 3)[1])
				}
			}
		}
	}
}

func (g *GPUInfo) detectVulkan() {
	if _, err := exec.LookPath("vulkaninfo"); err == nil {
		g.VulkanAvailable = true
	}
	if _, err := exec.LookPath("vkcube"); err == nil {
		g.VulkanAvailable = true
	}
}
