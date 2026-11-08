package scheduler

import (
	"fmt"
	"math"

	"gpu-coldstart-lab/internal/stages"
)

type Policy string

const (
	KeepWarm  Policy = "keep-warm"
	ColdStart Policy = "cold-start"
	Snapshot  Policy = "snapshot"
)

type Config struct {
	Policy      Policy
	Workers     int
	IdleTimeout float64
	ServiceTime float64
	FirstToken  float64
	Stages      stages.Durations
}

type Activity struct {
	Start float64
	End   float64
}

type Worker struct {
	Activities []Activity
	Available  float64
}

type Result struct {
	Policy     Policy
	Workers    []Worker
	Latencies  []float64
	ColdStarts int
	Restores   int
	End        float64
}

func Run(arrivals []float64, c Config) (Result, error) {
	if c.Policy != KeepWarm && c.Policy != ColdStart && c.Policy != Snapshot {
		return Result{}, fmt.Errorf("unknown policy %q", c.Policy)
	}
	if c.Workers < 1 || !finite(c.IdleTimeout) || c.IdleTimeout < 0 || !finite(c.ServiceTime) || c.ServiceTime <= 0 || !finite(c.FirstToken) || c.FirstToken < 0 || c.FirstToken > c.ServiceTime {
		return Result{}, fmt.Errorf("invalid worker count, timeout, service time, or first-token time")
	}
	r := Result{Policy: c.Policy, Workers: make([]Worker, c.Workers)}
	previous := -1.0
	for _, arrival := range arrivals {
		if !finite(arrival) || arrival < 0 || arrival < previous {
			return Result{}, fmt.Errorf("arrivals must be finite, non-negative and sorted")
		}
		previous = arrival
		best, bestStart, bestStartup, bestFinish := 0, 0.0, 0.0, math.Inf(1)
		for i, worker := range r.Workers {
			start := math.Max(arrival, worker.Available)
			startup := 0.0
			if c.Policy != KeepWarm && (len(worker.Activities) == 0 || start-worker.Available >= c.IdleTimeout) {
				if c.Policy == ColdStart {
					startup = stages.Total(c.Stages.Cold())
				} else {
					startup = stages.Total(c.Stages.Restore())
				}
			}
			finish := start + startup + c.ServiceTime
			if finish < bestFinish {
				best, bestStart, bestStartup, bestFinish = i, start, startup, finish
			}
		}
		w := &r.Workers[best]
		activation := c.Policy != KeepWarm && (len(w.Activities) == 0 || bestStart-w.Available >= c.IdleTimeout)
		if activation {
			r.ColdStarts++
			if c.Policy == Snapshot {
				r.Restores++
			}
		}
		w.Activities = append(w.Activities, Activity{Start: bestStart, End: bestFinish})
		w.Available = bestFinish
		r.Latencies = append(r.Latencies, bestStart-arrival+bestStartup+c.FirstToken)
		r.End = math.Max(r.End, bestFinish)
	}
	return r, nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
