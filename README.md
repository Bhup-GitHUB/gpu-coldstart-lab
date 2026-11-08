# gpu-coldstart-lab

A small, GPU-free simulation of model startup and GPU serving costs. All default durations, prices, and results are illustrative, not measurements. Go 1.22 or newer is the only requirement for the simulator; it uses the standard library.

```sh
make breakdown
make compare
make run-warm
make run-cold
make run-snapshot
```

`make compare` prints both startup paths and a policy comparison, then writes `results.csv`. Each run overwrites the output file. Run `go run ./cmd/sim -h` for all options, for example:

```sh
go run ./cmd/sim --policy all --seed 42 --workers 2 --bursts 10 \
  --burst-duration 60 --idle-gap 600 --rate 0.2 \
  --idle-timeout 30 --gpu-cost 2 --snapshot-cost 0.04
```

Edit `stages.json` or pass `--config path.json` to change stage durations in seconds. The cold path is container start, Python imports, weights to VRAM, CUDA initialization, and warmup. The restore path is container start plus snapshot restore. JSON is supported; YAML is not.

Default sample, seed 42, 115 requests, GPU price $2/hour and snapshot storage $0.04 per snapshot-hour:

| Policy | p50 TTFT | p99 TTFT | Cold starts | Restores | GPU hours | Idle GPU hours | Cost / 1,000 requests |
|---|---:|---:|---:|---:|---:|---:|---:|
| keep-warm | 0.200 s | 0.200 s | 0 | 0 | 3.3336 | 3.3017 | $57.98 |
| cold-start | 32.425 s | 50.200 s | 14 | 0 | 0.3264 | 0.1000 | $5.68 |
| snapshot | 0.200 s | 5.200 s | 14 | 14 | 0.2854 | 0.2340 | $6.12 |

These results depend on the traffic, durations, and prices. With no snapshot storage cost, faster restores can also make snapshot the cheapest policy.

## How the simulation works

Bursts contain Poisson arrivals at `--rate` requests per second, with `--idle-gap` seconds between bursts. A local seeded random generator makes the same input reproducible. All policies in a comparison use the same arrivals and observation window, from time zero through the end of the last burst or the last completed request, whichever is later.

Each worker serves one request at a time. An arrival is assigned to the worker that can finish it earliest, including queue and startup time. The warm worker's time to first token is `--first-token` seconds, and it remains busy for `--service-time` seconds. Keep-warm starts with all workers warm; the other policies start with all workers cold or with a snapshot available. After `--idle-timeout` seconds of inactivity, an on-demand worker scales down. A cold start counts every activation from scaled down, including snapshot restores; restores are also counted separately. Snapshots are assumed to exist before the simulation; creating one has no modeled latency or cost.

GPU time is billed from the beginning of startup through work and idle time until scale-down, clipped to the observation window. Startup is billed but not counted as idle. Keep-warm bills every worker for the entire window. Snapshot storage is billed separately for one stored snapshot per worker for the entire window. Cost per 1,000 requests includes GPU and snapshot storage charges. This is an illustrative accounting model, not a provider's pricing formula. Percentiles use the nearest-rank method.

## Optional GPU profiler

On a machine with a CUDA GPU, Python, PyTorch and a compatible vLLM installation, run:

```sh
python3 profiler/profile_vllm.py --output profile.json
# Optional: --model Qwen/Qwen2.5-0.5B-Instruct
go run ./cmd/sim --stages-from profile.json
```

The script launches a fresh Python process, times imports, vLLM initialization, and one-token offline generation. The default model may need to be downloaded. The process-launch measurement is used as `container_start`, but it does not measure container startup. `weights_to_vram` holds the combined vLLM initialization time, including CUDA setup and any built-in warmup; `cuda_init` is set to zero to avoid double counting. The `warmup` field contains the complete one-token generation call, including request overhead, not a streaming TTFT measurement. This script does not create or measure snapshots; `restore_snapshot` remains the illustrative value from the config. The separate simulator `--first-token` and `--service-time` values also remain illustrative unless supplied by the user. Model and measurement details are included in the profile JSON.

Modal describes [GPU memory snapshots](https://modal.com/blog/gpu-mem-snapshots) and its earlier [memory snapshot implementation](https://modal.com/blog/mem-snapshots). NVIDIA documents the [CUDA checkpoint and restore API](https://docs.nvidia.com/cuda/cuda-driver-api/cuda_driver_api/group__CUDA__CHECKPOINT.html). This project simulates the idea; it does not perform a real checkpoint or restore.
