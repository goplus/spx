# Web 端批量同步数据面

本文梳理 SPX Go 运行时和 Godot Web 运行时之间适合批量同步的数据面，并说明当前采用的共享传输缓冲实现边界。

## 同步方向

Web 端同步主要分为两条方向：

1. SPX -> Godot：Go 逻辑把游戏状态、命令和资源引用同步给 Godot。
2. Godot -> SPX：Godot 把输入、物理结果、生命周期事件和 UI 事件回传给 Go。

批量同步适合处理高频、结构固定、以 primitive 数据为主的内容。不适合直接传 Godot 对象指针、Go 指针、Variant 或复杂字符串对象。

## 已经批量同步的数据

### Sprite transform

当前每帧会收集 dirty sprite，并将以下字段序列化为 `[]float32` 后一次性发送：

- sprite id
- x, y
- rotation
- scale x, scale y
- render offset x, render offset y
- visible
- delete ids

对应入口：

- Go: `Game.updateSpriteProxies`
- Go: `SpriteSyncBuffer.Serialize`
- Go: `engine.SyncBatchUpdateSprites`
- Godot: `SpxSpriteMgr::batch_update_transforms`

这是最适合批量同步的主路径，因为它具备高频、多对象、固定字段、无需字符串的特点。

### Sprite visual

当前已有视觉批量缓冲，用于同步：

- sprite id
- render scale
- z index
- uv remap
- flags

对应入口：

- Go: `VisualSyncBuffer.Serialize`
- Go: `engine.SyncBatchUpdateVisuals`
- Godot: `SpxSpriteMgr::batch_update_visuals`

它适合继续扩展成更多 sprite 视觉属性，但 texture path、animation name 这类字符串字段不应混入这个 float buffer。

### Physics position pull

当前 Godot -> SPX 已有批量位置拉取：

- 输入：sprite id 列表
- 输出：x, y 列表

对应入口：

- Go: `Game.pullPhysicsPositions`
- Go: `engine.SyncBatchGetPositions`
- Godot raw method: `SpxSpriteMgr::batch_retrieve_positions`
- Godot Web raw export: `gdspx_sprite_batch_retrieve_positions`

Web 端直接走 raw return 快路径：Go 传入 sprite id fast-array，JS 借用 Godot wasm heap 上独立的 return ring slot，Godot 直接把 `float32` 位置写入该 slot，Go 再按 fast-array wrapper 解码。高层 `BatchRetrievePositions` 接口由 codegen 根据 raw 方法自动合成，不再手写 `GdArray -> GdArray` 的 Godot 导出。

### Input snapshot

Web 端输入查询已改为每帧快照优先：

- mouse world position
- mouse button bitset
- key pressed state cache
- action pressed / just pressed / just released per-frame cache
- axis value per-frame cache

对应入口：

- Go: `webffi.SyncWebInputSnapshot`
- Go: `webffi.WebInputMousePos` / `WebInputMouseState` / `WebInputKeyState`
- Go: `webffi.CachedActionBool` / `CachedActionAxis`
- Godot: `SpxInputMgr::write_snapshot`
- Godot Web raw export: `gdspx_input_write_snapshot`

当前 mouse position 和 mouse button 直接来自 Godot snapshot。键盘状态由 Web 端 key callback 维护本地 cache；Go API 层仍使用 action name，但 Web bridge 内部会将 action name 注册成数字 id，并对 action/axis 做同帧缓存。

action name 已增加注册表：

- Go 首次看到 action name 时调用 `GdspxInputActionID`。
- JS 将 action name 转成 Godot string 并调用 `gdspx_input_register_action`。
- Godot 保存 `action id -> StringName`。
- 后续 `IsActionPressed` / `IsActionJustPressed` / `IsActionJustReleased` / `GetAxis` 只传数字 id。

这把 action 字符串跨桥成本限制在首次注册，普通帧只走数字参数。

### Collision and trigger event queue

Web 端 collision / trigger callback 已改成 JS 侧 event queue：

- event type：collision enter / stay / exit，trigger enter / stay / exit
- self id low/high
- other id low/high

对应入口：

- JS: `GodotGdspx.queueContact`
- JS: `GodotGdspx.flushContactEvents`
- Go: `webffi.registerContactEventQueue`
- Go: `webffi.dispatchContactEvent`

Godot 触发事件时先写入 JS 数组队列，在 engine update / fixed update 前批量 flush 给 Go。若 Go direct handler 尚未注册，则回退到原有 `gdspx_dispatch` 逐条派发。

### Sprite physics config

Web 端新增 sprite physics command buffer，用于批量同步：

- velocity
- gravity / gravity scale
- mass
- physics mode
- drag / friction
- collision layer / mask
- trigger layer / mask
- collision enabled / trigger enabled

对应入口：

- Go: `PhysicsSyncBuffer.Serialize`
- Go: `engine.SyncBatchUpdatePhysics`
- Go Web wrapper: `WebSpriteBatchUpdatePhysics`
- Godot: `SpxSpriteMgr::batch_update_physics`
- Godot Web raw export: `gdspx_sprite_batch_update_physics`

buffer 格式为 `[count, cmd, spriteIdLowBits, spriteIdHighBits, a, b, reserved0, ...]`。sprite id 和 layer / mask / mode 这类整数使用 `float32` lane 的 raw bits 编码，避免按数值转成 `float32` 时丢失高位。polygon points、collider shape 参数这类可变长 payload 暂不进入该 fixed record buffer。

## 适合新增批量同步的数据

### Pen and debug draw

每帧绘制命令适合 command buffer：

- draw circle
- draw rect
- draw line
- pen move
- pen down / up
- stamp
- clear

这些命令天然是 append-only records，适合一帧 flush 一次。

### UI numeric layout

UI 的数值型属性可以批量：

- rect
- position
- size
- scale
- rotation
- visible
- interactable
- color
- font size

text、texture path、node path 仍然建议走字符串表或普通 bridge。

### Tilemap bulk operations

Tilemap 本身已有数组型批量接口空间，适合继续二进制化：

- place tiles
- erase tiles
- set layer offset
- collision points

但 tile texture path 应用字符串表或资源 id。

## 不适合批量共享内存的数据

以下内容不建议直接进入 ring buffer：

- Godot `Object*` 指针
- Go pointer / slice header / string header
- Godot `Variant`
- 需要 Godot 生命周期管理的 Node / Resource
- 资源加载、场景切换、动画创建
- 任意长度字符串
- 返回值依赖同步执行语义的查询调用

这些内容可以通过稳定 id、字符串表、资源表或现有 JS bridge 处理。

## 当前共享传输缓冲实现

浏览器里不能让 Go wasm 和 Godot wasm 直接共用同一个内部 heap。当前实现采用更可控的方式：

1. Go 将批量数据序列化为 `[]float32` 或其他 primitive slice。
2. Web FFI 优先向 JS 借用 Godot wasm heap 上的 ring slot。
3. Go 通过 `js.CopyBytesToJS` 直接写入这个 Godot wasm heap view。
4. JS 调用 Godot C++ 导出的 batch 函数，只传入 pointer + length。
5. 如果 ring slot 不可用，自动回退到旧的 `Uint8Array` scratch 再拷贝到 Godot wasm heap。
6. 对 Godot -> SPX 的固定 primitive 返回值，JS 使用独立的 return ring slot，避免覆盖同一次调用中的入参 slot。

该方案减少了一次中间 `Uint8Array -> Module.HEAPU8` 拷贝。在线程版 Godot Web 构建中，Godot wasm heap 的底层 buffer 是 `SharedArrayBuffer`；非线程构建中是普通 `ArrayBuffer`，但协议和调用路径保持一致。

## 后续优先级

暂无。
