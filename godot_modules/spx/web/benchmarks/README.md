# Web scalar bridge benchmark

`GdBool` and `GdFloat` inputs now cross JS → Wasm by value. IDs and other 64-bit values retain their exact integer transport; pointers and output parameters retain their existing ownership rules.

Run from this worktree with Emscripten activated:

```sh
python3 godot_modules/spx/web/benchmarks/run_scalars.py --node node
```

The script builds the actual generated wrappers and ABI utility from baseline `f263460539` and the working tree with Emscripten `-O3`. Identical C++ sinks replace manager work so the measurement isolates bridge overhead. It verifies float32 rounding, signed zero, infinities, NaN, booleans, full 64-bit ID lanes, and memory growth before timing seven alternating rounds of 200,000 calls.

Measured on Apple M4, macOS arm64, Node 24.20.0, Emscripten 3.1.62:

| Input | Before (million calls/s) | After | Speedup |
| --- | ---: | ---: | ---: |
| float | 2.43 | 16.23 | 6.69× |
| bool | 2.38 | 15.38 | 6.46× |
| 64-bit ID + float | 1.44 | 3.00 | 2.09× |
| 64-bit ID + four floats | 0.56 | 2.98 | 5.31× |

Each optimized input removes one pool acquire/release pair and two JS → Wasm calls. A synthetic group of 100 sprites updating rotation and visibility reduces crossings from 1,000 to 600 and median bridge time from 140.8 µs to 68.6 µs. Both versions perform zero `malloc` calls after warming their pools. These are bridge measurements, not game FPS or complete frame timings.

Raw measurements are in `scalars.m4.json`; rerun on the deployment browser and target hardware when estimating gameplay gains.
