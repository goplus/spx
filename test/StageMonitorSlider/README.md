# Slider Monitor

Use a monitor with `"mode": "slider"` or `"mode": 3`. Slider mode uses the
Scratch appearance without requiring `"style": "scratch"`.

`sliderMin` and `sliderMax` default to 0 and 100. Reversed bounds are swapped;
equal bounds create a fixed slider. `isDiscrete` defaults to true (step 1);
false uses step 0.01. Steps start at the minimum, including fractional minima.
The existing label, color, position, scale, and visibility settings apply.

Bind `val` to a writable variable (`score` or `getVar:score`), with an empty
`target` for a stage variable or a sprite name for a sprite-local variable.
Reporter methods and non-scalar fields cannot be slider targets. Go field types
are preserved: floats support decimals, integers truncate, strings receive the
numeric text, and `any` receives a float64. Integer overflow is rejected.

Dragging or clicking the track updates the variable and label on the next game
frame. A focused slider also accepts arrow keys and Home/End. Programmatic
changes update the slider on the normal monitor refresh. Values outside the
range keep their original text and variable value; only the thumb is clamped.
Non-numeric text places the thumb in the middle without changing the variable.
The slider works in the player; moving/resizing the monitor panel is separate.

Build the matching engine and bindings, then run from the repository root:

```sh
go run ./cmd/spx rune --path ./test/StageMonitorSlider
```

The example includes integer, decimal, dynamic-text, and sprite-local sliders.
The monitors on the right display the same variables independently. Press **R**
to reset stage values, **O** to set score to 200, **H** to hide/show its slider,
and **Space** to print the actual stage and sprite values.

Scratch references: `components/monitor/slider-monitor.jsx` and
`containers/slider-monitor.jsx` in scratch-gui.
