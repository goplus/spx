# Web scalar bridge benchmark

`GdBool`, `GdFloat`, `GdObj`, and `GdInt` inputs cross JS → Wasm by value. Go passes 64-bit inputs as two exact uint32 lanes; JS combines them into a signed BigInt for the Wasm call. Pointers and output parameters retain their existing ownership rules.

Run from this worktree with Emscripten activated:

```sh
python3 godot_modules/spx/web/benchmarks/run_scalars.py --node node
```

The script builds the actual generated wrappers and ABI utility from baseline `f497756e54` and the working tree with Emscripten `-O3`. Use `--baseline` and `--candidate` to compare other revisions. Identical C++ sinks replace manager work so the measurement isolates bridge overhead. It verifies float32 rounding, signed zero, infinities, NaN, booleans, complete signed 64-bit integers and IDs, argument order, output buffers, and memory growth before timing seven alternating rounds of 200,000 calls.

## Int64 inputs and export lookup simplification

Measured after all builds completed, on Apple M4 / Node 24.20.0 / Emscripten 3.1.62:

| Input | Before (million calls/s) | After | JS → Wasm calls |
| --- | ---: | ---: | ---: |
| ID + float | 2.97 | 3.21 | 3 → 1 |
| ID + four floats | 2.96 | 3.18 | 3 → 1 |
| two integers | 1.59 | 1.79 | 5 → 1 |
| ID + integer | 1.62 | 1.78 | 5 → 1 |
| two IDs + bool result | 0.95 | 1.08 | 7 → 3 |

The synthetic 100-sprite group drops from 600 to 200 crossings, with a 1.09× throughput increase. BigInt conversion still has a cost: fewer crossings do not imply a proportional speedup. The steady-state Wasm `malloc` count is zero before and after; this probe does not count JavaScript BigInt allocations.

An isolated comparison against round one (`66da2543af`) checks removal of the mirrored export cache: ID-only input cases remain within about 1%, while the case with a pooled bool result improves 1.08×. Unchanged float/bool controls vary by roughly 2%. These are single-host Node measurements, not browser gameplay or a cross-platform speed guarantee.

Raw results: `int64.m4.json` compares the complete change with `f497756e54`; `exports.m4.json` isolates the export lookup change. The `three-rounds` candidate label refers to the code in this commit; documentation and result files do not participate in the measurement.

## Earlier bool/float optimization

The results below compare `f263460539` with the earlier bool/float change, before int64 inputs moved to BigInt.

Measured on Apple M4, macOS arm64, Node 24.20.0, Emscripten 3.1.62:

| Input | Before (million calls/s) | After | Speedup |
| --- | ---: | ---: | ---: |
| float | 2.43 | 16.23 | 6.69× |
| bool | 2.38 | 15.38 | 6.46× |
| 64-bit ID + float | 1.44 | 3.00 | 2.09× |
| 64-bit ID + four floats | 0.56 | 2.98 | 5.31× |

Each optimized input removes one pool acquire/release pair and two JS → Wasm calls. A synthetic group of 100 sprites updating rotation and visibility reduces crossings from 1,000 to 600 and median bridge time from 140.8 µs to 68.6 µs. Both versions perform zero `malloc` calls after warming their pools. These are bridge measurements, not game FPS or complete frame timings.

Raw measurements are in `scalars.m4.json`; rerun on the deployment browser and target hardware when estimating gameplay gains.
