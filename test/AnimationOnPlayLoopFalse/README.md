# AnimationOnPlayLoopFalse

Manual demo for animation `onPlay` audio loop behavior.

## What to verify

- On startup, the Monkey plays `clapLoopFalse` with `animate "clapLoopFalse", true`.
- That animation uses `onPlay: { "play": "clap", "loop": false }`.
- The `true` in `animate(..., true)` only keeps the animation repeating across cycles.
- The loop behavior under test is the config value in `fAnimations.*.onPlay.loop`.
- Expected behavior: the clap sound is retriggered once per animation cycle, but each playback is still a one-shot sound.

## Controls

- `F`: start the `onPlay loop:false` demo
- `G`: switch to `onPlay loop:true` for comparison
- `S`: stop the current animation
