# Animation-Bound Audio

## Background

SPX can associate audio with an animation so that playback follows animation state instead of being started manually by game code. This is useful for frame animations whose sound timing is part of the asset definition.

## Configuration

Animation audio is configured in the sprite asset metadata. The binding selects a sound and a trigger policy for the animation. The runtime resolves the sound through the project's normal asset loading path.

## Semantics

### `onStart`

`onStart` audio plays when the animation starts from a stopped or inactive state. Re-selecting the same active animation does not repeatedly restart the sound.

### `onPlay`

`onPlay` audio follows explicit animation playback. It is intended for cases where every playback operation should trigger the associated sound.

## When an animation stops

Stopping or replacing an animation also stops audio owned by that animation. This prevents stale sounds from continuing after the visual state has changed. Audio started independently by game code is not owned by the animation and follows the regular sound API semantics.

## Relationship to issue #1376

The implementation aligns audio start and stop behavior with animation lifecycle events. In particular, it distinguishes initial activation from repeated playback and ensures cleanup when playback ends or the animation changes.

## When not to use animation-bound audio

Do not bind audio to an animation when the sound is controlled by gameplay state, must outlive the animation, or is shared across multiple unrelated animations. Use `play`, `playAndWait`, or the regular sound controls instead.

## Manual verification

The repository includes [`test/AnimationOnStartOnPlayAudio`](../../../../test/AnimationOnStartOnPlayAudio). Verify that:

1. start-bound audio triggers only on animation start;
2. play-bound audio triggers on the documented playback action;
3. switching or stopping animations cleans up owned audio;
4. unrelated audio continues according to the normal sound API.
