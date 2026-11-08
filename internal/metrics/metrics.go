package metrics

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"

	"gpu-coldstart-lab/internal/scheduler"
)

type Summary struct {
	Policy       scheduler.Policy
	Requests     int
	P50          float64
	P99          float64
	ColdStarts   int
	Restores     int
	GPUHours     float64
	IdleGPUHours float64
	SnapshotCost float64
	CostPer1000  float64
}

func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	index := int(math.Ceil(p*float64(len(v)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(v) {
		index = len(v) - 1
	}
	return v[index]
}

func Summarize(r scheduler.Result, horizon, timeout, price, snapshotPrice float64) Summary {
	s := Summary{Policy: r.Policy, Requests: len(r.Latencies), P50: Percentile(r.Latencies, 0.5), P99: Percentile(r.Latencies, 0.99), ColdStarts: r.ColdStarts, Restores: r.Restores}
	var billed, busy float64
	for _, worker := range r.Workers {
		for _, activity := range worker.Activities {
			busy += activity.End - activity.Start
		}
		if r.Policy == scheduler.KeepWarm {
			billed += horizon
			continue
		}
		end := -1.0
		for _, activity := range worker.Activities {
			if end < 0 {
				billed += activity.End - activity.Start
			} else if activity.Start <= end+timeout {
				billed += activity.End - end
			} else {
				billed += math.Min(timeout, horizon-end) + activity.End - activity.Start
			}
			end = activity.End
		}
		if end >= 0 {
			billed += math.Min(timeout, horizon-end)
		}
	}
	s.GPUHours = billed / 3600
	s.IdleGPUHours = math.Max(0, billed-busy) / 3600
	if r.Policy == scheduler.Snapshot {
		s.SnapshotCost = float64(len(r.Workers)) * horizon / 3600 * snapshotPrice
	}
	if s.Requests > 0 {
		s.CostPer1000 = (s.GPUHours*price + s.SnapshotCost) * 1000 / float64(s.Requests)
	}
	return s
}

func WriteCSV(path string, rows []Summary) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	err = w.Write([]string{"policy", "requests", "p50_ttft_seconds", "p99_ttft_seconds", "cold_starts", "restores", "gpu_hours", "idle_gpu_hours", "snapshot_cost", "cost_per_1000_requests"})
	for _, s := range rows {
		if err != nil {
			break
		}
		f := func(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
		err = w.Write([]string{string(s.Policy), strconv.Itoa(s.Requests), f(s.P50), f(s.P99), strconv.Itoa(s.ColdStarts), strconv.Itoa(s.Restores), f(s.GPUHours), f(s.IdleGPUHours), f(s.SnapshotCost), f(s.CostPer1000)})
	}
	w.Flush()
	if err == nil {
		err = w.Error()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}
	return nil
}
