package traffic

import (
	"fmt"
	"math"
	"math/rand"
)

type Config struct {
	Bursts        int
	BurstDuration float64
	IdleGap       float64
	Rate          float64
	Seed          int64
}

type Stream struct {
	Arrivals []float64
	End      float64
}

func Generate(c Config) (Stream, error) {
	if c.Bursts < 1 || !positive(c.BurstDuration) || !positive(c.Rate) || math.IsNaN(c.IdleGap) || math.IsInf(c.IdleGap, 0) || c.IdleGap < 0 {
		return Stream{}, fmt.Errorf("bursts, burst duration and rate must be positive; idle gap must be non-negative")
	}
	rng := rand.New(rand.NewSource(c.Seed))
	var stream Stream
	for i := 0; i < c.Bursts; i++ {
		start := float64(i) * (c.BurstDuration + c.IdleGap)
		for t := start + rng.ExpFloat64()/c.Rate; t < start+c.BurstDuration; t += rng.ExpFloat64() / c.Rate {
			stream.Arrivals = append(stream.Arrivals, t)
		}
	}
	stream.End = float64(c.Bursts)*c.BurstDuration + float64(c.Bursts-1)*c.IdleGap
	if len(stream.Arrivals) == 0 {
		return Stream{}, fmt.Errorf("traffic generated no requests; increase rate or burst duration")
	}
	return stream, nil
}

func positive(v float64) bool {
	return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v)
}
