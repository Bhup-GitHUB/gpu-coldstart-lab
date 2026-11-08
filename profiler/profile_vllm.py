import argparse
import json
import os
import subprocess
import sys
import tempfile
import time


def profile(model, started, output):
    ready = time.monotonic()
    import torch
    from vllm import LLM, SamplingParams

    imported = time.monotonic()
    if not torch.cuda.is_available():
        raise RuntimeError("CUDA GPU is not available")
    llm = LLM(model=model, trust_remote_code=False)
    loaded = time.monotonic()
    llm.generate(["Hello"], SamplingParams(max_tokens=1, temperature=0))
    finished = time.monotonic()
    data = {
        "container_start": ready - started,
        "python_imports": imported - ready,
        "weights_to_vram": loaded - imported,
        "cuda_init": 0,
        "warmup": finished - loaded,
        "measurement_notes": {
            "container_start": "Python process launch, not container startup",
            "weights_to_vram": "vLLM initialization including model load, CUDA initialization and any built-in warmup",
            "cuda_init": "Included in weights_to_vram; not measured separately",
            "warmup": "One-token offline generation after initialization; includes request overhead",
            "restore_snapshot": "Not measured; inherited from simulator config",
            "model": model,
        },
    }
    with open(output, "w", encoding="utf-8") as file:
        json.dump(data, file, indent=2)
        file.write("\n")


def main():
    parser = argparse.ArgumentParser(description="Measure vLLM startup on a CUDA GPU")
    parser.add_argument("--model", default="Qwen/Qwen2.5-0.5B-Instruct")
    parser.add_argument("--output", default="profile.json")
    parser.add_argument("--child-start", type=float, help=argparse.SUPPRESS)
    args = parser.parse_args()
    if args.child_start is not None:
        profile(args.model, args.child_start, args.output)
        return
    with tempfile.TemporaryDirectory() as directory:
        result = os.path.join(directory, "profile.json")
        started = time.monotonic()
        subprocess.run(
            [sys.executable, __file__, "--model", args.model, "--output", result, "--child-start", str(started)],
            check=True,
        )
        with open(result, encoding="utf-8") as source, open(args.output, "w", encoding="utf-8") as destination:
            destination.write(source.read())
    print(f"Profile written to {args.output}")


if __name__ == "__main__":
    main()
