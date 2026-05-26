# AnimationOnStartOnPlayAudio

Manual demo for animation `onStart` and `onPlay` audio behavior.

This scenario verifies the current fixed semantics:

- `onStart` is always one-shot playback.
- `onPlay` is always animation-bound replay.
- Animation audio config only needs `play`; `loop` is no longer part of this demo.
- Each animation cycle start should trigger `onStart` exactly once.
- When the sound is longer than one animation cycle, the current playback is cut off and restarted at the next cycle.

## What to verify

- On startup, the Monkey plays `clapTruncate` with `animate "clapTruncate", true`.
- `clapTruncate` and `clapComplete` both have `onStart.play = chomp`.
- Expected `onStart` behavior: pressing `F` or `G` should play one `chomp` immediately, and each later animation loop should trigger one new `chomp`.
- `clapTruncate` runs a 0.5s animation cycle, shorter than the `0.88s` `clap` sound.
- Expected behavior: each new animation cycle restarts the sound immediately, so the previous `clap` is audibly cut off before it finishes.
- `clapComplete` is the comparison case: its cycle is longer than the sound, so `clap` should keep looping within the same animation cycle, then get cut off and restarted only when the next animation cycle begins.
- `longWalk` is a much longer `~3.92s` sound copied from `.tmp/niu-run` for boundary testing.
- `longWalkTruncate` keeps the same 0.5s cycle, so the long sound should be cut off almost immediately at every loop boundary.
- `longWalkComplete` slows the animation to about `4.33s` per cycle, so the long sound should run through most of the cycle, then get cut and restarted at the next boundary.

## Controls

- `F`: start the short-cycle truncation demo and verify repeating one-shot `onStart`
- `G`: switch to the longer-cycle continuous-loop demo and verify repeating one-shot `onStart`
- `H`: start the long-walk short-cycle boundary demo
- `J`: start the long-walk long-cycle boundary demo
- `S`: stop the current animation
