package ai

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ModelManager struct {
	modelsDir string
	models    []AIModel
}

func NewModelManager(modelsDir string) *ModelManager {
	return &ModelManager{modelsDir: modelsDir}
}

func (m *ModelManager) ListModels() ([]AIModel, error) {
	var result []AIModel
	entries, err := os.ReadDir(m.modelsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			continue
		}
		path := filepath.Join(m.modelsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		model := AIModel{
			Name:       strings.TrimSuffix(entry.Name(), ".gguf"),
			Path:       path,
			FileSizeMB: int(info.Size() / (1024 * 1024)),
		}
		if qt := parseQuantization(entry.Name()); qt != "" {
			model.Quantization = qt
		}
		result = append(result, model)
	}
	m.models = result
	return result, nil
}

func (m *ModelManager) RegisterModel(path, name, license string) (AIModel, error) {
	info, err := os.Stat(path)
	if err != nil {
		return AIModel{}, fmt.Errorf("model file not found: %w", err)
	}
	model := AIModel{
		Name:         name,
		Path:         path,
		FileSizeMB:   int(info.Size() / (1024 * 1024)),
		License:      license,
	}
	if qt := parseQuantization(name); qt != "" {
		model.Quantization = qt
	}
	m.models = append(m.models, model)
	return model, nil
}

func (m *ModelManager) VerifyModel(name string) (AIModel, string, error) {
	var model AIModel
	found := false
	for _, mod := range m.models {
		if mod.Name == name {
			model = mod
			found = true
			break
		}
	}
	if !found {
		return AIModel{}, "", fmt.Errorf("model %q not found", name)
	}
	if model.SHA256 == "" {
		return model, "no stored checksum", nil
	}
	f, err := os.Open(model.Path)
	if err != nil {
		return model, "", fmt.Errorf("open model: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return model, "", fmt.Errorf("read model: %w", err)
	}
	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != model.SHA256 {
		return model, fmt.Sprintf("checksum mismatch: expected %s, got %s", model.SHA256[:16], actual[:16]), nil
	}
	return model, "verified", nil
}

func (m *ModelManager) DeleteModel(name string) error {
	var model AIModel
	var idx int = -1
	for i, mod := range m.models {
		if mod.Name == name {
			model = mod
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("model %q not found", name)
	}
	if err := os.Remove(model.Path); err != nil {
		return fmt.Errorf("delete model file: %w", err)
	}
	m.models = append(m.models[:idx], m.models[idx+1:]...)
	return nil
}

func (m *ModelManager) EstimateVRAM(name string) int {
	for _, mod := range m.models {
		if mod.Name == name {
			return mod.FileSizeMB * 2
		}
	}
	return 0
}

func parseQuantization(name string) string {
	for _, token := range strings.Split(strings.ToLower(name), "-") {
		token = strings.TrimPrefix(token, "q")
		token = strings.TrimPrefix(token, "q8")
		if len(token) >= 2 && (token[0] >= '1' && token[0] <= '8') {
			return "Q" + strings.ToUpper(token)
		}
	}
	return ""
}

func (m *ModelManager) VerifyCommitSHA(name, commitSHA string) bool {
	for _, mod := range m.models {
		if mod.Name == name {
			return mod.SHA256 == commitSHA
		}
	}
	return false
}

func humanSize(mb int) string {
	if mb > 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return strconv.Itoa(mb) + " MB"
}
