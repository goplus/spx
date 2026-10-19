# SPX Script Scheduling

[中文](../../../zh/dev/engine/scheduling.md)

## Overview

SPX schedules scripts cooperatively. A script runs until it yields or finishes;
the scheduler then lets other runnable scripts proceed. Loop boundaries,
explicit waits, redraw requests, and condition events determine when a script
can continue.

This document describes the current SPX runtime, including frame phases, loop
rounds, and the limits of capture and input replay. These rules
apply to all projects using the corresponding APIs.

## Execution ownership and completion

Each admitted script uses a Go goroutine to preserve its stack. `runMu` serializes
script slices. `yield` releases execution ownership; a resumed script must acquire it
again before continuing. `Current()` identifies that owner, not the goroutine
calling an API. `Task.Owner` and `Thread.Owner` identify the logical task owner.

Time, frame, Join, and external-task waits release execution ownership.
`WaitMainThread` is a synchronous engine call and retains the script slice while
waiting. Cancellation can discard a queued request; a running request must return
before its calling script exits. Engine-thread waits service engine requests
without advancing future frames.

Channel receives use managed external tasks and publish their result only after
the script resumes normally. A wait that exits through cancellation does not
write its result to the caller.
`JoinFirstSlice(thread)` waits for the target's first yield or completion.
Join and Latch unregister waiters on completion or cancellation. `Execute` waits
for task completion even when admission rejects the task or cancellation prevents
its callback from starting.

Cancellation, script completion, and runtime drain are separate stages. Both
`Cancel` and `Stop` request termination and wake suspended scripts; they cannot
interrupt arbitrary blocking Go code. Reset barriers wait for scripts, external
work, and final panic handling. The host uses `RunAfterStop(nil)` to wait for
all registered work; scripts use the non-waiting stop operations. A `WaitToDo`
worker that exits through `Goexit` cancels and wakes its script so the script can
clean up.

### Stop operations

| API | Behavior |
| --- | --- |
| `Stop(thread)` / `StopIf` | Request cancellation without waiting for cleanup. |
| `StopCurrent` | Exit the current coroutine through procedure boundaries. |
| `StopThisScript` | Exit the nearest custom procedure and resume its caller; without a procedure boundary, end the coroutine. |
| `StopAll` | Request cancellation of all registered coroutines without waiting. |
| `RunAfterStop(call)` | From the host, close admission, request cancellation, drain scripts and workers, then run a callback if non-nil. Managed callers are rejected before changing state. |

`Stop` names script operations, `Cancel` signals cancellation, and internal drain
waits cover cleanup. `ErrAbortThread` remains the control-flow signal for exiting
the current coroutine.

### Shutdown barriers

`RunAfterStop` is a host-only, synchronous barrier with no timeout or return value.
It closes admission, cancels registered scripts, waits for scripts, workers, and
final panic handling to exit, then invokes the cleanup callback. Scripts use the
non-waiting stop operations; their cancellation contexts remain independent of
this host barrier.

If engine-thread cleanup exits the waiting goroutine before drain completes,
admission remains closed. A later successful barrier can finish cleanup and
restore admission. The callback runs only after all registered work has exited.

### Callbacks and synchronous waits

`RegisterInvocation` runs synchronously on the creator after the thread is
registered with the scheduler and before the task executes. Its returned cleanup
runs when the task exits, including cancellation before execution.
`RegisterInvocation` must not wait for tasks in its batch: execution requires all
of that batch's registration callbacks to return.
Admission rejection returns a canceled, completed thread without starting a
goroutine or invoking registration and execution callbacks. Both Join waits
therefore complete immediately, including for managed callers.

For non-empty batches, `BatchWaitFirstSlice` requires a managed caller: its return
depends on reacquiring script ownership after the batch's first slices. Host calls
are rejected before any task is registered. `BatchAsync` and `BatchWaitDone` also
accept host callers; empty batches return immediately in every mode.

Scripts, workers, registration callbacks, cleanup, final panic handlers, shutdown
callbacks, and callbacks holding exclusive script ownership cannot synchronously
drain. Such calls panic with `ErrReentrantWait` before canceling tasks or changing
admission. Exclusive callbacks also cannot synchronously
execute scripts or block on Join / Latch, since those scripts cannot acquire execution ownership.
Completed waits may return.

Shutdown callbacks run with admission closed but without holding the admission
lock. They may call `StopAll`; new tasks are rejected. Normal return, panic, and
`Goexit` all restore admission.

Cancellation wakes suspended scripts without a future `Update`. While waiting for
the barrier or drain, the native engine thread services main-thread jobs without
advancing frames or script rounds. Drain can finish at the same frame number;
this does not guarantee completion within one frame's time budget. Web background
waits park the Go goroutine. A synchronous JS callback that needs later events
must move its wait to a separate goroutine and return to the host.

Shutdown waits for running engine calls and callbacks to finish; reacquiring
script ownership also remains cooperative. External workers must observe their
work context or use the operation's cancellation mechanism. Arbitrary blocking
code and circular dependencies can still prevent cleanup.

### Concurrency invariants

- `schedulerMu` protects runnable state and the shared current-job queue. Update
  and engine waiters select and remove a job under that lock, then execute it
  after unlocking. The engine serializes Update calls, which own the deferred
  and next-round queues; queues have no separate locks.
- Admission readers may register concurrently; `pendingSetups` is atomic and
  delays reopening until all registration callbacks finish. Callbacks run outside
  the admission lock. The atomic epoch rejects requests from before a stop,
  even after admission reopens.
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
  lock to avoid missed notifications. Admission stays closed during the wait;
  the last thread or worker to unregister notifies all waiters, including after
  final panic handling. The script's completion channel is not a substitute.

## Frame phases

An engine update advances the frame clock once. It may contain several script
rounds; starting another round does not advance the frame number or timer.

1. Cache engine input and freeze the same kind of input batch for live input,
   recording, or replay. After startup scripts first yield or finish, sample
   condition events using that input before advancing the frame clock. Input
   preparation and condition matching share one `RunBetweenScripts` phase.
2. Advance the clock and handle capture-key requests. Freeze timers due at this
   phase, register matched condition handlers, then route this frame's input and
   timers. Finish registration before dispatching startup handlers.
3. Process runnable coroutines and eligible loop rounds.
4. Synchronize visual state needed by captures, dispatch queued capture requests,
   and finalize input-session frame completion. The Web host waits for its
   render fence before reading the canvas.

### Input and timer frame boundaries

Edges sampled before the input snapshot cutoff belong to this frame; later input
belongs to the next. Routing order is mouse edges, mouse movement, keys, then due
timers; this does not impose a global order on user handlers. Timers registered
or reactivated by `ResetTimer` during dispatch are checked in a subsequent batch.
Timers are routed at the start of `OnEngineUpdate`. Pending audio and completed
animations are still flushed by `logicLoop`; that work is not guaranteed to precede
timer registration or callbacks.

The batch holds script execution ownership while completing `handleEvent` routing
and handler registration before this frame's script scheduling. Input no longer
passes through the 16-item `Game.events` channel and its asynchronous receive worker.
Handlers still use `BatchAsync`; normally, `Update` waits for runnable handlers
to yield or finish. `Wait`, `WaitNextFrame`, and external-task waits can defer their
continuation to future frames. Cancellation, stop rules, reentry policies, and
early watchdog exit still apply: registration does not guarantee that every handler
has started or finished. Click hit testing and the 50 ms throttle remain in effect.

Batch registration does not mean one callback per input edge. Key registrations
use `IgnoreWhileRunning`, rejecting new calls while an earlier invocation remains
active. An idle `OnAnyKey` or multi-key `OnKey` registration accepts only its first
matching event in a batch.

When upgrading, remove `EventQueuePolicy` from Go configuration and
`eventQueuePolicy` from JSON. The Go field has been removed; the current project
JSON loader ignores the old field. Frame batches no longer offer `DropNewest`,
`DropOldest`, or `Block` queue policies. Handler admission still follows the rules above.

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

## Event registration snapshots

The event Manager appends registrations under its lock. Snapshots may share their
backing array with subsequent appends. `Snapshot` and `SnapshotStartOnce` return
shallow, read-only views: callers must not assign existing elements. Capacity is
limited to the current length, so appending to a snapshot uses separate storage.
Objects referenced by a Sink, including its Handler, retain their existing
mutability; snapshots do not synchronize those objects or guarantee owner lifetime.

Deleting an owner copies the surviving registrations. Reset only drops the
Manager's slice references. Neither operation changes published elements, so old
snapshots retain their registration order and contents across append, deletion,
and Reset. The first `SnapshotStartOnce` still closes start registration until Reset.

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
| Thread admission and lifecycle, stop and drain barriers | `internal/coroutine/lifecycle.go`, `shutdown.go` |
| Callback reentry guards, engine-thread calls, frame-boundary callbacks | `internal/coroutine/callback.go`, `mainthread.go` |
| Timed and channel waits, worker lifecycle | `internal/coroutine/wait.go` |
| Frame waits, loop rounds, redraw budget | `internal/coroutine/frame.go` |
| Wait-job processing and scheduler statistics | `internal/coroutine/update.go`, `stats.go` |
| Control-flow integration | `internal/engine/coro.go` |
| Frame phase ordering | `internal/engine/engine.go`, `runtime_engine.go` |
| Frame input sampling, unified input and timer dispatch | `runtime_input.go`, `runtime_replay_input.go` |
| Condition registration, sampling, and matched-handler dispatch | `runtime_conditions.go` |
| Visibility checks and redraw notification | `sprite_render.go` and the relevant visual operations |

`UpdateStats()` returns statistics from the latest update, with durations in
milliseconds. `queue`, `waitJob`, and `waitKind` are private scheduler details.

Frame-boundary callbacks exclude script execution while servicing queued
engine main-thread calls. A script waiting on such a call must not block
condition sampling. This phase does not advance loop or frame waiters.
Frame-boundary callbacks run on the caller. External native event dispatch uses
a managed coroutine and services only engine-thread jobs while waiting, so a
synchronous dispatch cannot wait for a future frame.

## Capture and input replay

During an active game, `Snapshot` queues a request for dispatch after visual-state
synchronization at the end of the frame. A configured `captureKey` requests
screenshots from recorded or replayed key presses at the same input ticks.
These requests do not fix how much script work occurs before capture.
See [Web capture](web_capture.md) for host integration.

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

These rules cover frame input, timers, ordinary loops, explicit waits, redraw boundaries, and
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
- Live input, recording, and replay share frame batches; routing and registration
  finish before script scheduling, and timers use the advanced clock.
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
`runtime_engine_test.go`, `runtime_replay_test.go`, `runtime_event_dispatch_native_test.go`,
`sprite_clone_collision_test.go`, `internal/coroutine/loop_test.go`, and
`internal/coroutine/order_test.go`.
Cancellation and restart coverage is in `internal/coroutine/thread_test.go`,
`internal/coroutine/worker_test.go`, `internal/coroutine/join_test.go`,
`internal/coroutine/handler_test.go`, and `internal/engine/execute_test.go`.
WASM compilation does not replace browser runtime verification.

## Reference implementation

- [Scratch Sequencer.stepThreads](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/sequencer.js): rounds, work budget, and redraw boundaries.
- [Scratch Runtime.startHats / _step](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/runtime.js): condition and script execution phases.
- [Scratch Clock](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/io/clock.js): the shared frame clock.
