# AnimationOnStartOnPlayAudio

Manual demo for animation `onStart` and `onPlay` audio behavior.

This scenario verifies the current fixed semantics:

- `onStart` only supports one-shot playback (`loop=false`).
- `onPlay` only supports animation-bound replay (`loop=true`).
- Each new `animate` call should trigger `onStart` exactly once.
- When the sound is longer than one animation cycle, the current playback is cut off and restarted at the next cycle.

## What to verify

- On startup, the Monkey plays `clapTruncate` with `animate "clapTruncate", true`.
- `clapTruncate` and `clapComplete` both have `onStart.play = chomp` with `loop = false`.
- Expected `onStart` behavior: pressing `F` or `G` should play one `chomp` immediately, and that `chomp` must not repeat on later animation loops.
- `clapTruncate` runs a 0.5s animation cycle, shorter than the `0.88s` `clap` sound.
- Expected behavior: each new animation cycle restarts the sound immediately, so the previous `clap` is audibly cut off before it finishes.
- `clapComplete` is the comparison case: its cycle is longer than the sound, so `clap` should keep looping within the same animation cycle, then get cut off and restarted only when the next animation cycle begins.

## Controls

- `F`: start the short-cycle truncation demo and verify one-shot `onStart`
- `G`: switch to the longer-cycle continuous-loop demo and verify one-shot `onStart`
- `S`: stop the current animation
