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

## Execution ownership and completion

Each script uses a Go goroutine to preserve its stack. `runMu` serializes script
slices. `Yield` releases execution ownership; a resumed script must acquire it
again before continuing. `Current()` identifies that owner, not the goroutine
calling an API.

Time, frame, Join, and external-task waits release execution ownership.
`WaitMainThread` is a synchronous engine call and retains the script slice while
waiting. Cancellation can discard a queued request; a running request must return
before its calling script exits. Engine-thread waits service engine requests
without advancing future frames.

Channel receives use managed external tasks and publish their result only after
the script resumes normally. A wait that exits through cancellation does not
write its result to the caller.
Join and Latch unregister waiters on completion or cancellation. `Execute` waits
for task completion even when admission rejects the task or cancellation prevents
its callback from starting.

Cancellation, script completion, and runtime drain are separate stages. Both
`Cancel` and `Stop` request termination and wake suspended scripts; they cannot
interrupt arbitrary blocking Go code. Reset barriers wait for scripts, external
work, and final panic handling. Ordinary scripts may call `StopAllAndWait`, which
excludes the calling script. A `WaitToDo` worker that exits through `Goexit`
cancels and wakes its script so the script can clean up.

### Stop operations

| API | Behavior |
| --- | --- |
| `Stop(thread)` / `StopIf` | Request cancellation without waiting for cleanup. |
| `StopCurrent` | Exit the current coroutine through procedure boundaries. |
| `StopThisScript` | Exit the nearest custom procedure and resume its caller; without a procedure boundary, end the coroutine. |
| `StopAll` | Request cancellation of all registered coroutines without waiting. |
| `StopAllAndWait` | Request cancellation and wait for scripts and workers; a managed caller is also canceled but excluded from the wait. |
| `RunAfterStopAll` | Close admission, request cancellation, drain work, then run a callback as an external reset barrier. |

`Stop` names script operations, `Cancel` signals cancellation, and internal drain
waits cover cleanup. `ErrAbortThread` remains the control-flow signal for exiting
the current coroutine.

### Callbacks and synchronous waits

`Setup` runs synchronously on the creator after registration and before the task
starts. Its returned cleanup runs when the task exits, including cancellation
before execution. `Setup` must not wait for tasks in its batch: execution requires
all of that batch's setup callbacks to return.

Workers, setup, cleanup, final panic handlers, shutdown callbacks, and callbacks
holding exclusive script ownership cannot synchronously drain. Such calls panic
with `ErrReentrantWait` before canceling tasks or changing admission. Exclusive
callbacks also cannot synchronously execute scripts or block on Join / Latch,
since those scripts cannot acquire execution ownership. Completed waits may return.

Shutdown callbacks run with admission closed but without holding the admission
lock. They may call `StopAll`; new tasks are rejected. Normal return, panic, and
`Goexit` all restore admission. A drain timeout keeps admission closed until a
later successful `RunAfterStopAll`. While waiting for the shutdown lock or drain
notifications, the native engine thread services main-thread jobs without
advancing frames or script rounds. Web has no native main-thread queue to service;
background waits park the Go goroutine so the JS event loop can process asynchronous
callbacks. A synchronous JS callback must still move work that depends on later JS
events to a separate goroutine and return to the host.

The timeout bounds waiting for drain notifications, not the entire call. Lock
acquisition, running callbacks, and reacquiring script ownership remain
cooperative. Arbitrary blocking Go code, circular user dependencies, and setup
waiting for its own task can still prevent progress.

### Concurrency invariants

- Waiter registration and the blocked state are published together, with lock
  order `schedulerMu` → `waiterSet.mu`. Closing a set transfers its waiters;
  waking them after unlocking avoids reversing this order.
- Thread completion makes Join waiters runnable before canceling the context,
  closing the completion channel, and removing scheduler state. Update must not
  observe an empty runnable set between the target and its waiters.
- External work registers before starting and unregisters when the worker exits.
  Canceling its script cannot decrement the work count early;
  reset must wait for the work itself to finish.
- Drain waits check their predicate and subscribe under the lifecycle registry
  lock to avoid missed notifications. Actual unregistration notifies waiters,
  including after final panic handling; the script's completion channel is not
  a substitute.

## Frame phases

An engine update advances the frame clock once. It may contain several script
rounds; starting another round does not advance the frame number or timer.

1. Cache engine input and resolve the current recording or replay tick. After
   startup scripts first yield or finish, sample condition events using that
   input before advancing the frame clock.
2. Advance the clock. Dispatch matched condition handlers, input-session events
   and capture-key requests, and run the startup or due frame callbacks.
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
4. **Preserve script order across frames.** Deferred waits and loop continuations
   are checked in script registration order. Each keeps its resume conditions.
   Runnable scripts, including new handlers, yield or finish before another
   queued script resumes. Engine main-thread calls remain available throughout.
   Restarting an active event handler preserves its position in this order while
   assigning a new Thread ID. Invocations started after the previous one stopped
   or completed take a new registration position.

## Condition events

1. **Evaluate conditions before advancing the clock.** After startup scripts
   first yield or finish, sample conditions before each frame's clock update.
   Dispatch the matched handlers after that update, without evaluating the
   conditions again. Preserve existing target order and finish all evaluations
   before starting handlers. Input predicates observe the current session tick,
   including its final tick before replay pauses.
2. **Trigger condition events on rising edges.** A sustained true condition
   does not start another handler. Suspend that event's condition sampling while
   its handler is active, then resume edge detection when it finishes.
   Predicates must return promptly and must not call waiting APIs.

## Implementation structure

| Responsibility | Location |
| --- | --- |
| Admission and task lifecycle, stop and drain barriers | `internal/coroutine/lifecycle.go`, `shutdown.go` |
| Callback reentry guards, engine-thread calls and waits | `internal/coroutine/callback.go`, `mainthread.go` |
| Frame waits, loop rounds, redraw budget, frame-boundary callbacks | `internal/coroutine/frame.go` |
| Wait-job processing and scheduler statistics | `internal/coroutine/update.go` |
| Control-flow integration | `internal/engine/coro.go` |
| Frame phase ordering | `internal/engine/engine.go`, `runtime_engine.go` |
| Condition registration, sampling, and matched-handler dispatch | `runtime_conditions.go` |
| Visibility checks and redraw notification | `sprite_render.go` and the relevant visual operations |

Frame-boundary callbacks exclude script execution while servicing queued
engine main-thread calls. A script waiting on such a call must not block
condition sampling. This phase does not advance loop or frame waiters.
Frame-boundary callbacks run on the caller. External native event dispatch uses
a managed coroutine and services only engine-thread jobs while waiting, so a
synchronous dispatch cannot wait for a future frame.

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

These rules cover ordinary loops, explicit waits, redraw boundaries, and
condition events. Startup, cancellation, broadcasts, and cloning retain their
respective lifecycle contracts; ordering waits across frames does not impose a
global FIFO order on external events or all coroutines. The implementation is
in Go and requires no Godot C++ interface changes.

The loop and condition-event rules follow the corresponding Scratch mechanisms.
They do not imply complete Scratch VM compatibility or identical iteration
counts across runs or platforms.
For example, an asynchronous broadcast's new runnable handlers may execute before
older scripts still in the wait queue. Scratch instead appends new threads to its
existing thread list. The scheduler watchdog checks time only after regaining
control; it does not provide preemption or a hard timeout.

## Regression coverage

- Nonvisual loops can run multiple rounds per frame; redraws do not truncate
  the current round's other scripts.
- Explicit waits retain their boundaries, and `Warp` retains its yield rules.
- Deferred waits retain registration order; unexpired waits do not block
  eligible scripts.
- Scripts continue in later frames after budget exhaustion and remain cancellable.
- Predicates and handlers observe the states before and after clock advancement,
  respectively; slow frames preserve this phase ordering.
- Condition events preserve rising edges, non-reentrancy, and owner isolation.
- Pending main-thread calls cannot deadlock the sampling phase.
- Concurrent drain waiters all receive completion notifications; reset waits for
  the last external worker to finish.
- Concurrent Latch opens resume scripts once and publish Done before returning.
- Synchronous engine calls preserve script slices; cancellation cannot let a
  running call outlive the reset barrier.
- Resumed scripts publish receive results; canceled Join waiters release their references.
- Active handler restarts preserve their position; stopped or completed handlers register anew.
- Callback drain and exclusive callback script reentry fail promptly; engine
  drain waits keep servicing workers' main-thread cleanup.
- Worker `Goexit` stops and wakes its script; abnormal shutdown callbacks restore admission.

```sh
go test -race ./internal/coroutine ./internal/engine ./internal/core/runtime .
go test ./...
GOOS=js GOARCH=wasm go test -c -o /tmp/spx-scheduling.test.wasm .
```

Core regression coverage is in `runtime_scratch_scheduler_test.go`,
`runtime_events_test.go`, `runtime_events_order_test.go`,
`sprite_clone_collision_test.go`, `internal/coroutine/loop_test.go`, and
`internal/coroutine/order_test.go`.
Cancellation and restart coverage is in `internal/coroutine/wait_cancel_test.go`,
`join_cancel_test.go`, `handler_order_test.go`, and `internal/engine/execute_test.go`.
WASM compilation does not replace browser runtime verification.

## Reference implementation

- [Scratch Sequencer.stepThreads](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/sequencer.js): rounds, work budget, and redraw boundaries.
- [Scratch Runtime.startHats / _step](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/runtime.js): condition and script execution phases.
- [Scratch Clock](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/io/clock.js): the shared frame clock.
