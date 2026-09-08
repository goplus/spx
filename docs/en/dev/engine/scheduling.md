# SPX Script Scheduling

[中文](../../../zh/dev/engine/scheduling.md)

## Overview

SPX schedules scripts cooperatively. A script runs until it yields or finishes;
the scheduler then lets other runnable scripts proceed. Loop boundaries,
explicit waits, redraw requests, and condition events determine when a script
can continue.

This document describes the current SPX runtime, including frame phases, loop
rounds, and the limits of fixed-frame capture and input replay. These rules
apply to all projects using the corresponding APIs.

## Frame phases

An engine update advances the frame clock once. It may contain several script
rounds; starting another round does not advance the frame number or timer.

1. After startup scripts first yield or finish, sample condition events before
   advancing the frame clock.
2. Advance the clock and cache engine input. Dispatch matched condition
   handlers, process an active input session, and run the startup or due frame
   callbacks.
3. Process runnable coroutines and eligible loop rounds.
4. Synchronize visual state needed by captures, dispatch queued capture requests,
   and finalize input-session frame completion. The Web host waits for its
   render fence before reading the canvas.

## Loops, redraws, and waits

1. **Distinguish frames from execution rounds.** In normal mode, `Forever`,
   `Repeat`, `RepeatUntil`, and `WaitUntil` yield to other scripts. Another round
   may run in the same frame if no redraw is pending and budget remains. Actual
   work determines the iteration count.
2. **Handle redraws at round boundaries.** A redraw allows the other scripts in
   the current round to proceed, then defers the next round to the next frame.
   Visible sprite transforms, costumes, effects, visibility changes, backdrops,
   visible bubbles, and pen drawing notify the scheduler through a shared
   entry point. Hidden sprite transforms do not request redraws.
3. **Keep explicit waits distinct.** Time waits and `WaitNextFrame` have their
   own resume conditions. Ordinary loop rounds use a wall-clock budget of 75%
   of the default 30 Hz interval, or 25 ms. This budget controls admission of
   another round; it does not interrupt the current round. `Warp` uses a
   separate 500 ms cooperative yield budget.

## Condition events

1. **Evaluate conditions before advancing the clock.** After startup scripts
   first yield or finish, sample conditions before each frame's clock update.
   Dispatch the matched handlers after that update, without evaluating the
   conditions again. Preserve existing target order and finish all evaluations
   before starting handlers.
2. **Trigger condition events on rising edges.** A sustained true condition
   does not start another handler. Suspend that event's condition sampling while
   its handler is active, then resume edge detection when it finishes.
   Predicates must return promptly and must not call waiting APIs.

## Implementation structure

| Responsibility | Location |
| --- | --- |
| Frame waits, loop rounds, redraw budget, main-thread state reads | `internal/coroutine/frame.go` |
| Wait-job processing and scheduler statistics | `internal/coroutine/update.go` |
| Control-flow integration | `internal/engine/coro.go` |
| Frame phase ordering | `internal/engine/engine.go`, `runtime_engine.go` |
| Condition registration, sampling, and matched-handler dispatch | `runtime_conditions.go` |
| Visibility checks and redraw notification | `sprite_render.go` and the relevant visual operations |

Frame-boundary state reads exclude script execution while servicing queued
engine main-thread calls. A script waiting on such a call must not block
condition sampling. This phase does not advance loop or frame waiters.

## Fixed-frame capture and input replay

During an active game, `AtFrame` schedules a callback for an engine-session
frame. `Snapshot` queues a request for dispatch after visual-state synchronization
at the end of the frame. These APIs select a frame boundary; they do not fix
how much script work occurs before it. See [Web capture](web_capture.md) for
host integration.

[Input recording and replay](input_replay.md) fix the input stream, logical
timestep, and script random seed. They do not fix or record the number of
script rounds per frame. The loop scheduler still checks real elapsed time.
If visible state depends on how much loop work fits into that budget, different
machine loads can produce different images at the same input tick, even with
the same build and recording.

Regression scenarios should use explicit frame or logical-time boundaries for
observable progress instead of depending on budget-limited iteration counts.
Regenerating a baseline after a scheduling change can account for changed
semantics, but it does not remove variation caused by wall-clock scheduling.

## Scope

These rules cover ordinary loops, redraw boundaries, and condition events.
Startup, cancellation, broadcasts, and cloning retain their respective
lifecycle contracts; the scheduler does not impose a global FIFO order on all
coroutines. The implementation is in Go and requires no Godot C++ interface
changes.

The loop and condition-event rules follow the corresponding Scratch mechanisms.
They do not imply complete Scratch VM compatibility or identical iteration
counts across runs or platforms.

## Regression coverage

- Nonvisual loops can run multiple rounds per frame; redraws do not truncate
  the current round's other scripts.
- Explicit waits retain their boundaries, and `Warp` retains its yield rules.
- Scripts continue in later frames after budget exhaustion and remain cancellable.
- Predicates and handlers observe the states before and after clock advancement,
  respectively; slow frames preserve this phase ordering.
- Condition events preserve rising edges, non-reentrancy, and owner isolation.
- Pending main-thread calls cannot deadlock the sampling phase.

```sh
go test -race ./internal/coroutine ./internal/engine ./internal/core/runtime .
go test ./...
GOOS=js GOARCH=wasm go test -c -o /tmp/spx-scheduling.test.wasm .
```

Core regression coverage is in `runtime_scratch_scheduler_test.go`,
`runtime_events_test.go`, `runtime_events_order_test.go`, and
`internal/coroutine/loop_test.go`. WASM compilation does not replace browser
runtime verification.

## Reference implementation

- [Scratch Sequencer.stepThreads](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/sequencer.js): rounds, work budget, and redraw boundaries.
- [Scratch Runtime.startHats / _step](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/runtime.js): condition and script execution phases.
- [Scratch Clock](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/io/clock.js): the shared frame clock.
