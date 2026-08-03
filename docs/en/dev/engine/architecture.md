## Architecture

### 0. Overall architecture

SPX consists of the XGo game runtime, platform bindings, and the SPX fork of Godot. Game logic is written in XGo/SPX, while rendering, audio, input, physics, and platform integration are provided by the engine.

The Go-to-engine boundary differs by platform. Native platforms use GDExtension/native bindings. Web builds combine Go WASM with an Emscripten-built Godot runtime and JavaScript bridge code.

### 1. PC platforms

Desktop builds support two primary workflows:

- interpreted development through `spx run`;
- native runtime execution through `spx runnative`, editor execution, or exported desktop packages.

SPX communicates with Godot through generated native bindings. Runtime assets and the matching engine binary must be installed before native execution.

### 2. Web platform

Web builds contain a Go WASM module, a Godot Web export, and JavaScript glue. SPX supports four modes: `normal`, `worker`, `minigame`, and `miniprogram`. The same mode must be used while preparing, building, and exporting assets.

Normal mode runs the host integration on the browser main thread. Worker mode moves the engine workload to a worker and uses message-based bridges where direct browser access is unavailable. Mini-game and mini-program modes add platform-specific adapters.

### 3. Android platform

Android exports use the Godot Android template together with the SPX runtime and generated bindings. Building from source requires the Android SDK, NDK, JDK, and the SPX Godot fork.

### 4. iOS platform

iOS exports use the Godot iOS template and SPX runtime. Local exports require the Apple toolchain and signing configuration. The build orchestration prepares the required engine artifacts before the project export step.
