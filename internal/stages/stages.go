package stages

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Durations struct {
	ContainerStart  float64 `json:"container_start"`
	PythonImports   float64 `json:"python_imports"`
	WeightsToVRAM   float64 `json:"weights_to_vram"`
	CUDAInit        float64 `json:"cuda_init"`
	Warmup          float64 `json:"warmup"`
	RestoreSnapshot float64 `json:"restore_snapshot"`
}

type Stage struct {
	Name    string
	Seconds float64
}

func Load(path string, base Durations) (Durations, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, err
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return base, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, stage := range append(base.Cold(), base.Restore()...) {
		if math.IsNaN(stage.Seconds) || math.IsInf(stage.Seconds, 0) || stage.Seconds < 0 {
			return base, fmt.Errorf("invalid duration for %s", stage.Name)
		}
	}
	return base, nil
}

func (d Durations) Cold() []Stage {
	return []Stage{{"container_start", d.ContainerStart}, {"python_imports", d.PythonImports}, {"weights_to_vram", d.WeightsToVRAM}, {"cuda_init", d.CUDAInit}, {"warmup", d.Warmup}}
}

func (d Durations) Restore() []Stage {
	return []Stage{{"container_start", d.ContainerStart}, {"restore_snapshot", d.RestoreSnapshot}}
}

func Total(path []Stage) float64 {
	var total float64
	for _, stage := range path {
		total += stage.Seconds
	}
	return total
}
