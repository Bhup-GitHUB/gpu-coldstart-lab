package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"gpu-coldstart-lab/internal/metrics"
	"gpu-coldstart-lab/internal/scheduler"
	"gpu-coldstart-lab/internal/stages"
	"gpu-coldstart-lab/internal/traffic"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	policy := flag.String("policy", "all", "keep-warm, cold-start, snapshot, or all")
	config := flag.String("config", "stages.json", "JSON stage durations in seconds")
	profile := flag.String("stages-from", "", "JSON profiler output overriding configured durations")
	breakdown := flag.Bool("breakdown", false, "print startup breakdown only")
	seed := flag.Int64("seed", 42, "traffic random seed")
	bursts := flag.Int("bursts", 10, "number of traffic bursts")
	burstDuration := flag.Float64("burst-duration", 60, "seconds per burst")
	idleGap := flag.Float64("idle-gap", 600, "seconds between bursts")
	rate := flag.Float64("rate", 0.2, "Poisson arrivals per second during a burst")
	workers := flag.Int("workers", 2, "maximum number of GPU workers")
	timeout := flag.Float64("idle-timeout", 30, "seconds before an idle worker scales down")
	price := flag.Float64("gpu-cost", 2, "cost per GPU hour")
	service := flag.Float64("service-time", 1, "seconds a request occupies a worker")
	firstToken := flag.Float64("first-token", 0.2, "seconds to first token on a warm worker")
	output := flag.String("output", "results.csv", "CSV output path")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}
	if math.IsNaN(*price) || math.IsInf(*price, 0) || *price < 0 {
		return fmt.Errorf("GPU cost must be finite and non-negative")
	}
	durations, err := stages.Load(*config, stages.Durations{})
	if err != nil {
		return err
	}
	if *profile != "" {
		durations, err = stages.Load(*profile, durations)
		if err != nil {
			return err
		}
	}
	fmt.Println("Stage durations in seconds (configured values are illustrative unless measured):")
	printBreakdown("Cold start", durations.Cold())
	printBreakdown("Snapshot restore", durations.Restore())
	if *breakdown {
		return nil
	}
	stream, err := traffic.Generate(traffic.Config{Bursts: *bursts, BurstDuration: *burstDuration, IdleGap: *idleGap, Rate: *rate, Seed: *seed})
	if err != nil {
		return err
	}
	policies := []scheduler.Policy{scheduler.KeepWarm, scheduler.ColdStart, scheduler.Snapshot}
	if *policy != "all" {
		policies = []scheduler.Policy{scheduler.Policy(*policy)}
	}
	var runs []scheduler.Result
	horizon := stream.End
	for _, p := range policies {
		r, err := scheduler.Run(stream.Arrivals, scheduler.Config{Policy: p, Workers: *workers, IdleTimeout: *timeout, ServiceTime: *service, FirstToken: *firstToken, Stages: durations})
		if err != nil {
			return err
		}
		runs = append(runs, r)
		horizon = math.Max(horizon, r.End)
	}
	var rows []metrics.Summary
	for _, r := range runs {
		rows = append(rows, metrics.Summarize(r, horizon, *timeout, *price))
	}
	if err := metrics.WriteCSV(*output, rows); err != nil {
		return err
	}
	fmt.Printf("\n%d requests; seed %d; observation window %.2f s; GPU cost %.2f/hour\n", len(stream.Arrivals), *seed, horizon, *price)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "POLICY\tP50 TTFT (s)\tP99 TTFT (s)\tCOLD STARTS\tRESTORES\tGPU HOURS\tIDLE GPU HOURS\tCOST / 1K")
	for _, s := range rows {
		fmt.Fprintf(w, "%s\t%.3f\t%.3f\t%d\t%d\t%.4f\t%.4f\t%.2f\n", s.Policy, s.P50, s.P99, s.ColdStarts, s.Restores, s.GPUHours, s.IdleGPUHours, s.CostPer1000)
	}
	w.Flush()
	fmt.Printf("\nResults written to %s\n", *output)
	return nil
}

func printBreakdown(name string, path []stages.Stage) {
	fmt.Printf("\n%s\n", name)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "STAGE\tSECONDS")
	for _, stage := range path {
		fmt.Fprintf(w, "%s\t%.3f\n", stage.Name, stage.Seconds)
	}
	fmt.Fprintf(w, "TOTAL\t%.3f\n", stages.Total(path))
	w.Flush()
}
