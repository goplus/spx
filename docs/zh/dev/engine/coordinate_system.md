# SPX 坐标系统与 Godot 边界

本文定义 SPX 运行时涉及的坐标空间、各空间的所有者和转换边界。目标是让同一个坐标值只在一个明确的位置转换，避免 Go 与 Godot 两侧重复翻转或补偿。

## 1. 坐标空间

### 1.1 SPX 世界坐标

SPX 脚本、Go 运行时和公开 API 默认使用 SPX 世界坐标：

- 原点位于舞台中心。
- X 轴向右为正。
- Y 轴向上为正。
- 长度单位是逻辑像素；服装的原始像素需要先除以 `bitmapResolution`。

精灵的 `X/Y`、速度、力、射线、碰撞查询、相机位置、画笔位置和鼠标世界坐标都必须以这一坐标系进入或离开 Go 运行时。

### 1.2 资产坐标

图片和服装配置使用资产坐标：

- 原点位于图片左上角。
- X 轴向右为正。
- Y 轴向下为正。
- 单位是图片原始像素。

`costume.center` 属于资产坐标。资产数据进入 SPX 时，在 `costume.renderAnchorInSPX` 中归一化为 SPX 局部坐标。它不是 SPX 与 Godot 的世界坐标转换。

设图片原始宽高为 `W/H`，资产中心为 `cx/cy`，位图分辨率为 `r`，则服装锚点相对图片几何中心的 SPX 局部坐标为：

```text
assetAnchorSPX = (
    (cx - W / 2) / r,
    (H / 2 - cy) / r,
)
```

结合精灵 pivot 和运行时服装缩放后，渲染偏移为：

```text
renderOffsetSPX = -(assetAnchorSPX + pivot) * costumeScale
```

这个值是精灵逻辑原点到服装渲染中心的局部向量，仍然属于 SPX 坐标。

### 1.3 SPX 局部渲染坐标

精灵根节点的位置始终表示逻辑/物理位置。`renderOffsetSPX` 只用于渲染子树：

```text
SpxSprite (CharacterBody2D)       # 逻辑位置、旋转、朝向和物理根
├── RenderRoot (Node2D)           # renderOffsetSPX
│   ├── Anim2D (AnimatedSprite2D) # 纹理和逐帧偏移
│   └── VisibleNotifier2D         # 可见性通知
├── Collider2D                    # 实体碰撞，直接属于 CharacterBody2D
└── Area2D                        # 触发区域
    └── Trigger2D                 # 触发形状
```

这样有以下约束：

- `SpxSprite.position` 等于 SPX 逻辑位置经过边界转换后的值，不含服装偏移。
- `RenderRoot.position` 等于服装局部渲染偏移经过边界转换后的值。
- 显式碰撞中心相对逻辑根定位，不需要减去渲染偏移。
- 自动碰撞中心来源于图片 alpha 边界，需要加上局部渲染偏移后再挂到物理根。
- `CollisionShape2D` 必须直接服务于 `CharacterBody2D`，不能放到 `AnimatedSprite2D` 下。

精灵旋转或左右翻转时，`RenderRoot` 作为根节点的子节点自然继承根变换。Go 侧需要世界渲染中心的功能（例如印章和逻辑边界查询）使用同一局部偏移并应用根变换，不能直接把偏移加到世界位置。

### 1.4 Godot 画布坐标

Godot 2D 世界坐标的 Y 轴向下。SPX 与 Godot 的向量转换统一位于仓库中的 `godot_modules/spx/spx_coordinate.h`：

```text
spxToGodot(x, y) = (x, -y)
godotToSPX(x, y) = (x, -y)
```

该转换是自身的逆变换。任何跨 Go/Godot 边界的世界或局部空间 `Vec2` 都只能在 Godot 模块入口/出口转换一次。

## 2. 所有权规则

| 数据 | 内部表示 | 转换位置 |
| --- | --- | --- |
| 脚本位置、速度、力、法线 | SPX 世界坐标 | Godot manager 入口/出口 |
| 相机、鼠标、射线、寻路点 | SPX 世界坐标 | Godot manager 入口/出口 |
| 碰撞中心和多边形顶点 | SPX 局部坐标 | Godot collision 入口 |
| `costume.center` | 资产坐标 | Go `costume` 元数据层 |
| `renderOffset` | SPX 局部渲染坐标 | Go 计算，Godot `RenderRoot` 应用 |
| 动画逐帧 offset | Godot 渲染局部坐标 | Godot 资源/渲染层 |
| UI 控件位置 | 左上角 UI 坐标 | `internal/ui.ViewToUI` |

核心规则：

1. Go/SPX 业务层不为 Godot 手工翻转 Y。
2. Godot manager 收到 SPX `Vec2` 后立即转换，返回前转换回 SPX。
3. 资产坐标只在资产元数据进入 SPX 时归一化。
4. UI 坐标转换不属于 Godot 世界坐标边界，不能复用世界坐标转换函数。
5. 尺寸、半径和均匀缩放不是位置向量，不做 Y 轴翻转。

## 3. 主要数据流

### 3.1 SPX 驱动精灵

```text
脚本 X/Y
  -> Go transformComponent（SPX 世界坐标）
  -> transform sync position（仍为 SPX 世界坐标）
  -> SpxSpriteMgr::set_transform / batch_update_transforms
  -> spx_to_godot_vec2
  -> SpxSprite.position（Godot 世界坐标）
```

服装渲染偏移通过同一条 transform sync 作为独立字段传递，进入 Godot 后设置到 `RenderRoot.position`，不再修改根节点位置。

### 3.2 Godot 物理回传

```text
SpxSprite.position（Godot 世界坐标）
  -> SpxSpriteMgr position getter / batch position pull
  -> godot_to_spx_vec2
  -> Go transformComponent（SPX 世界坐标）
```

因为物理根不再包含渲染偏移，Go 侧不需要 `revertRenderOffset`。

### 3.3 碰撞形状

- 显式碰撞：`logicalPosition + rootTransform(colliderPivot)`。
- 自动碰撞：`logicalPosition + rootTransform(renderOffset + alphaBoundsCenter)`。
- 形状中心和多边形顶点在 Godot collision 边界统一转换 Y。

## 4. 允许存在的其他坐标转换

以下转换与 SPX/Godot 世界坐标反射不同，应保持独立且名称明确：

- `costume.renderAnchorInSPX`：资产左上角坐标转 SPX 局部锚点。
- `internal/ui.ViewToUI`：舞台中心坐标转 UI 左上角坐标。
- 动画 payload/frame offset：图片帧在图集或资源内部的局部布局。
- 相机 view/world 变换：缩放和相机偏移，不是 Y 轴约定转换。

## 5. 修改检查清单

新增或修改坐标相关 API 时检查：

1. 参数属于哪个坐标空间？
2. 调用链中是否已经转换过？
3. Godot manager 是否在入口/出口调用统一 helper？
4. 值是位置/方向向量，还是尺寸/标量？
5. 渲染偏移是否只影响 `RenderRoot`，没有污染物理根位置？
6. 普通调用和 batch 调用是否使用相同规则？
7. 是否覆盖正负 Y、往返转换、旋转/翻转渲染偏移和物理位置回传测试？
