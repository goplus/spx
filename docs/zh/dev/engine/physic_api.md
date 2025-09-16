# Spx物理API设计

## 设计理念

采用CodeMonkey风格的极简API设计，只保留最核心的8个方法，覆盖80%的游戏物理需求。简洁、直观、易用。


## 完整API概览

```go
// PhysicsMode 物理模式
type PhysicsMode int

// 全局命名空间，可以移除命名后缀
const (
	NoPhysics        PhysicsMode = 0 // 纯视觉，无碰撞，无物理  eg:装饰品，scratch 中的sprite
	KinematicPhysics PhysicsMode = 1 // 代码控制移动，有碰撞检测： eg:玩家
	DynamicPhysics   PhysicsMode = 2 // 受物理影响（重力、碰撞） eg:物品
	StaticPhysics    PhysicsMode = 3 // 静态物理，无重力，不响应物理事件，但是会影响其他物理体 eg： 墙壁
)
// 注意:
// Static 模式不支持 SetVelocity ,SetGravity,AddImpulse 方法，调用了等于没有调用，IsOnFloor 方法返回 false
// NoPhysics 模式不支持 SetVelocity ,SetGravity,AddImpulse 方法，调用了等于没有调用，IsOnFloor 方法返回 false

type ColliderType = int

const (
    // 参数:
    //   width: 碰撞器宽度，单位：像素
    //   height: 碰撞器高度，单位：像素
    //
    // 特点:
    //   - 矩形碰撞器，以对象中心为基准
    //
    // 使用场景:
    //   - 大部分角色和物体（箱子、平台等）
    //   - 建筑物和墙壁
    //   - 规则形状的游戏对象
	RectCollider    ColliderType = 0 

    // 参数:
    //   radius: 圆形半径，单位：像素
    //
    // 特点:
    //   - 圆形碰撞器，以对象中心为圆心
    //   - 适合球形物体或需要平滑碰撞的对象
    //   - 碰撞检测相对高效
    //
    // 使用场景:
    //   - 球类物体（足球、弹珠等）
    //   - 圆形敌人或角色
    //   - 需要平滑滚动的物体
    //   - 爆炸范围检测
	CircleCollider  ColliderType = 1

    // 参数:
    //   radius: 胶囊半径，单位：像素
    //   height: 胶囊高度，单位：像素
    //
    // 特点:
    //   - 胶囊形状，上下为半圆，中间为矩形
    //   - 适合人形角色，避免卡在斜坡上
    //   - 提供更自然的碰撞体验
    //
    // 使用场景:
    //   - 人形角色（玩家、NPC）
    //   - 需要在斜坡上平滑移动的对象
    //   - 高瘦形状的物体
	CapsuleCollider ColliderType = 2

    // 参数:
    //   vertices: 顶点坐标数组，格式：[x1, y1, x2, y2, ...]
    //
    // 特点:
    //   - 自定义形状碰撞器
    //   - 可以精确匹配复杂形状
    //   - 计算开销相对较高
    //
    // 使用场景:
    //   - 不规则地形
    //   - 复杂形状的平台
    //   - 精确形状匹配的特殊物体
    //
    // 注意事项:
    //   - 顶点需要按逆时针顺序排列
    //   - 建议控制顶点数量以保证性能
	PolygonCollider ColliderType = 3
)


// 物理控制（4个核心）
func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode) 
func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64) 
func (p *SpriteImpl) SetGravity(gravity float64) 
func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64) 

// 碰撞器（2个）
// Rect(width, height float64) 
// Circle(radius float64) 
// Capsule(radius, height float64) 
// Polygon(params []float64) 
func (p *SpriteImpl) SetColliderParams(type ColliderType, params []float64) 
func (p *SpriteImpl) SetColliderPivot(offsetX, offsetY float64) 

// 查询方法（4个实用）
func (p *SpriteImpl) IsOnFloor() bool
func (p *Game) Raycast(fromX, fromY, toX, toY float64, ignoresSprites []Sprite = nil) (hit bool, hitX, hitY float64, sprite Sprite)
func (p *Game) IntersectRect(posX, posY, width, height float64) (sprites []Sprite)// 矩形碰撞检测
func (p *Game) IntersectCircle(posX, posY, radius float64) (sprites []Sprite) // 圆形碰撞检测

// getter 
func (p *SpriteImpl) Velocity() (velocityX, velocityY float64)
func (p *SpriteImpl) Gravity() float64
func (p *SpriteImpl) PhysicsMode() PhysicsMode
func (p *SpriteImpl) ColliderParams() (ColliderType,[]float64)
func (p *SpriteImpl) ColliderPivot() (offsetX, offsetY float64)

// tilemap // -1 表示任意layer
func (p *Game) PlaceTiles(posXYs []float64, texture string, layer int64 = 0)
func (p *Game) PlaceTile(posX, posY float64, texture string, layer int64 = 0)
func (p *Game) EraseTile(posX, posY float64, layer int64 = -1)
func (p *Game) GetTile(posX, posY float64, layer int64 = -1) string

// config
type projConfig struct {
	Physics  bool    `json:"physics"`  // 是否开启物理模式 默认 false，兼容Scratch
	GlobalGravity  float64 `json:"globalGravity"`  // 全局重力缩放系数，默认 1
	GlobalFriction float64 `json:"globalFriction"` // 全局摩擦力缩放系数，默认 1
	GlobalAirDrag  float64 `json:"globalAirDrag"`  // 全局空气阻力缩放系数，默认 1
}
```

## 计划
### 一期
```go
func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode)
func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64) 
func (p *SpriteImpl) SetGravity(gravity float64) 
func (p *SpriteImpl) IsOnFloor() bool
```

### 二期
```go
func (p *SpriteImpl) SetColliderParams(type ColliderType, params []float64) 
func (p *SpriteImpl) SetColliderPivot(offsetX, offsetY float64) 
func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64) 
func (p *Game) Raycast(fromX, fromY, toX, toY float64, ignoresSprites []Sprite = nil) (hit bool, hitX, hitY float64, sprite Sprite)
func (p *Game) IntersectRect(posX, posY, width, height float64) (sprites []Sprite)// 矩形碰撞检测
func (p *Game) IntersectCircle(posX, posY, radius float64) (sprites []Sprite) // 圆形碰撞检测
```

### 三期
Tilemap 相关功能
```go
func (p *Game) PlaceTiles(posXYs []float64, texture string, layer int64 = 0)
func (p *Game) PlaceTile(posX, posY float64, texture string, layer int64 = 0)
func (p *Game) EraseTile(posX, posY float64, layer int64 = 0)
func (p *Game) GetTile(posX, posY float64, layer int64 = 0) string
```

### 四期
开放全局摩擦力，空气阻力等配置， layer，mask等高级概念


## 兼容性考虑
1. 场景配置需要增加一个 Physics 属性 ，默认为false，兼容scratch 模式
   如果为true, 开启物理模式
2. 如果物理模式开启，所有精灵默认PhysicsMode = DynamicPhysics(有重力效果), 如果希望其他模式，需要在builder中配置 或 在spx脚本代码中进行设置
3. 如果物理模式没有开启，所有精灵默认PhysicsMode = NoPhysics ,无物理效果，和scratch 保持一致
4. 全局有个重力系数的配置globalGravity，SetGravity的值，其实会受到这个值的影响， eg: globalGravity = 9.8 , SetGravity(2.0), 则实际重力为 9.8 * 2.0 = 19.6

## 核心API定义

### 1. 物理控制接口（4个核心方法）

```go
// SetPhysicsMode 设置物理模式
// 
// 参数:
//   mode: 物理模式
//     - DynamicPhysics: 受重力和外力影响，有碰撞检测（类似Unity isKinematicPhysics=false）
//     - KinematicPhysics: 只能通过代码控制移动，有碰撞检测（类似Unity isKinematicPhysics=true）
//     - NoPhysics: 纯视觉对象，无碰撞，无物理效果（类似Unity无Collider）
//     - StaticPhysics: 静态物理，无重力，不响应物理事件，但是会影响其他物理体（类似Unity isStaticPhysics=true）
// 
//
// 使用场景:
//   - 玩家角色使用DynamicPhysics模式
//   - NPC巡逻使用KinematicPhysics模式  
//   - 背景装饰使用NoPhysics模式
//   - 墙壁和地面使用StaticPhysics模式
func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode)
```

```go
// SetVelocity 设置速度向量
//
// 参数:
//   velocityX: 水平速度，单位：像素/秒，正值向右，负值向左
//   velocityY: 垂直速度，单位：像素/秒，正值向上，负值向下（注意：重力会影响Y轴速度）
//
//
// 注意事项:
//   - DynamicPhysics模式：设置的速度会受重力和外力影响而改变
//   - KinematicPhysics模式：精确按设定速度移动，不受重力影响  
//   - NoPhysics模式：直接用于transform移动，不调用物理引擎
//
// 常用模式:
//   - 角色水平移动：SetVelocity(speed * direction, Velocity().Y)
//   - 停止移动：SetVelocity(0, Velocity().Y)
func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64)
```

```go
// SetGravity 设置重力缩放
//
// 参数:
//   gravity: 重力缩放因子
//     - 1.0: 正常重力
//     - 0.0: 无重力（漂浮效果）
//     - 2.0: 双倍重力（快速下落）
//     - -1.0: 反重力（向上飞行）
//     - 0.3: 水中效果（缓慢下沉）
//
//
// 注意事项:
//   - 只对DynamicPhysics模式有效
//   - KinematicPhysics和NoPhysics模式会忽略重力设置
//   - 可以在运行时动态调整，实现各种环境效果
//
// 使用场景:
//   - 正常角色: SetGravity(1.0)
//   - 游泳状态: SetGravity(0.2)  
//   - 飞行道具: SetGravity(0.0)
//   - 快速下落: SetGravity(3.0)
func (p *SpriteImpl) SetGravity(gravity float64)
```

```go
// AddImpulse 施加瞬间冲量
//
// 参数:
//   impulseX: 水平冲量，单位：像素/秒，正值向右推，负值向左推
//   impulseY: 垂直冲量，单位：像素/秒，正值向上推，负值向下推
//
//
// 特点:
//   - 瞬间改变速度，不是持续施力
//   - 只对DynamicPhysics模式有效
//   - 会叠加到当前速度上：新速度 = 当前速度 + 冲量
//
// 常用场景:
//   - 跳跃: AddImpulse(0, -400)
//   - 击退: AddImpulse(-300, -100)  
//   - 爆炸推力: AddImpulse(200, -200)
//   - 弹跳: AddImpulse(0, -300)
//
// 技巧:
//   - 可以用来实现二段跳、冲刺、击飞等效果
//   - 与SetVelocity的区别：AddImpulse是累加，SetVelocity是替换
func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64)
```

### 2. 碰撞器接口（2个碰撞器方法）

```go
// SetColliderParams 设置碰撞器参数
//
// 参数:
//   type: 碰撞器类型
//     - RectCollider: 矩形碰撞器
//       params: [width, height] - 宽度和高度，单位：像素
//     - CircleCollider: 圆形碰撞器  
//       params: [radius] - 半径，单位：像素
//     - CapsuleCollider: 胶囊碰撞器
//       params: [radius, height] - 半径和高度，单位：像素
//     - PolygonCollider: 多边形碰撞器
//       params: [x1, y1, x2, y2, ...] - 顶点坐标数组，单位：像素
//
// 特点:
//   - 统一的碰撞器设置接口，支持所有碰撞器类型
//   - 可以在运行时动态切换碰撞器类型和大小
//   - 参数数组长度必须与碰撞器类型匹配
//   - 设置后立即生效，影响物理检测和碰撞
//
// 使用场景:
//   - 角色状态变化时调整碰撞器（站立/蹲下）
//   - 物品破碎后改变碰撞形状
//   - 建筑物建造时设置精确碰撞器
//   - 车辆形态变化时切换碰撞器类型
//
// 示例:
//   - 设置矩形碰撞器：SetColliderParams(RectCollider, []float64{32, 48})
//   - 设置圆形碰撞器：SetColliderParams(CircleCollider, []float64{16})
//   - 设置胶囊碰撞器：SetColliderParams(CapsuleCollider, []float64{12, 32})
//   - 设置三角形碰撞器：SetColliderParams(PolygonCollider, []float64{0, -16, -16, 16, 16, 16})
//
// 注意事项:
//   - 参数数组长度错误会导致设置失败
//   - PolygonCollider的顶点需要按逆时针顺序排列
//   - 碰撞器尺寸建议比显示图像略小，避免视觉上的穿透感
//   - 频繁切换碰撞器类型可能影响性能
func (p *SpriteImpl) SetColliderParams(type ColliderType, params []float64)
```

```go
// SetColliderPivot 设置碰撞器中心点偏移
//
// 参数:
//   offsetX: 水平偏移，单位：像素，正值向右，负值向左
//   offsetY: 垂直偏移，单位：像素，正值向下，负值向上
//
// 特点:
//   - 调整碰撞器相对于精灵中心的位置
//   - 默认值为(0, 0)，表示碰撞器在精灵中心
//   - 对所有类型的碰撞器都有效
//   - 可以在运行时动态调整
//
// 使用场景:
//   - 精灵图片重心不在中央时的调整
//   - 角色蹲下时将碰撞器下移
//   - 武器碰撞器位置微调
//   - 车辆底盘碰撞器下移到轮子位置
//
// 示例:
//   - 角色头部碰撞器：SetColliderPivot(0, -10)
//   - 角色脚部碰撞器：SetColliderPivot(0, 15)
//   - 武器前端碰撞器：SetColliderPivot(20, 0)
func (p *SpriteImpl) SetColliderPivot(offsetX, offsetY float64)
```

### 3. 查询接口（4个实用方法）

```go
// IsOnFloor 检查是否在地面上
//
// 返回:
//   bool: true表示脚下有可站立的表面，false表示在空中
//
// 检测逻辑:
//   - 从对象底部向下发射短距离射线
//   - 检测是否碰到静态物体或平台
//   - 只有DynamicPhysics和KinematicPhysics模式返回有意义的值
//
// 使用场景:
//   - 判断是否可以跳跃：if IsOnFloor() { AddImpulse(...) }
//   - 播放落地音效：if IsOnFloor() && wasInAir { playLandSound() }
//   - 切换动画状态：if IsOnFloor() { playIdleAnim() } else { playFallAnim() }
//   - 限制二段跳：if IsOnFloor() { jumpCount = 0 }
//
// 注意事项:
//   - NoPhysics模式始终返回false
//   - 在斜坡上也会返回true
//   - 检测距离很短，轻微离地就会返回false
func (p *SpriteImpl) IsOnFloor() bool
```

```go
// Raycast 发射射线检测
//
// 参数:
//   fromX: 射线起点X坐标，单位：像素
//   fromY: 射线起点Y坐标，单位：像素
//   toX: 射线终点X坐标，单位：像素
//   toY: 射线终点Y坐标，单位：像素
//
// 返回:
//   hit: 是否击中对象
//   hitX: 击中点X坐标，如果没击中则为0
//   hitY: 击中点Y坐标，如果没击中则为0
//   sprite: 被击中的精灵对象，如果没击中则为nil
//
// 检测特性:
//   - 检测第一个碰到的对象（按距离排序）
//   - 忽略NoPhysics模式的对象
//   - 忽略射线起点所在的对象（避免自己检测自己）
//
// 使用场景:
//   - 武器射击检测：hit, hitX, hitY, enemy := player.Raycast(gunPos.X, gunPos.Y, targetPos.X, targetPos.Y)
//   - 视线检测：hit, _, _, _ := enemy.Raycast(enemyPos.X, enemyPos.Y, playerPos.X, playerPos.Y)
//   - 地面检测：hit, _, _, _ := player.Raycast(playerPos.X, playerPos.Y, playerPos.X, playerPos.Y + 50)
//   - 寻路避障：hit, _, _, _ := unit.Raycast(currentPos.X, currentPos.Y, nextPos.X, nextPos.Y)
//   - 激光特效：从起点到击中点画线
//
// 技巧:
//   - 可以用很短的射线实现精确的接触检测
//   - 可以用很长的射线实现远距离视线检测
//   - 结合循环可以实现扇形范围检测
func (p *Game) Raycast(fromX, fromY, toX, toY float64, ignoresSprites []Sprite = nil) (hit bool, hitX, hitY float64, sprite Sprite)
```

```go
// IntersectRect 矩形区域碰撞检测
//
// 参数:
//   posX: 检测区域中心X坐标，单位：像素
//   posY: 检测区域中心Y坐标，单位：像素
//   width: 检测区域宽度，单位：像素
//   height: 检测区域高度，单位：像素
//
// 返回:
//   sprites: 与检测区域重叠的所有精灵对象列表
//
// 特点:
//   - 检测指定矩形区域内的所有碰撞对象
//   - 忽略NoPhysics模式的对象
//   - 忽略调用者自身
//   - 返回所有重叠的对象，按距离排序
//
// 使用场景:
//   - 范围攻击检测：爆炸、魔法范围等
//   - 区域触发器：进入特定区域时触发事件
//   - 批量物体检测：检测附近所有敌人
//   - 建筑放置检测：确保建筑位置无障碍物
func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite
```

```go
// IntersectCircle 圆形区域碰撞检测
//
// 参数:
//   posX: 检测区域中心X坐标，单位：像素
//   posY: 检测区域中心Y坐标，单位：像素
//   radius: 检测区域半径，单位：像素
//
// 返回:
//   sprites: 与检测区域重叠的所有精灵对象列表
//
// 特点:
//   - 检测指定圆形区域内的所有碰撞对象
//   - 忽略NoPhysics模式的对象
//   - 忽略调用者自身
//   - 返回所有重叠的对象，按距离排序
//
// 使用场景:
//   - 圆形范围攻击：手榴弹爆炸、AOE魔法
//   - 感知范围检测：敌人警戒范围
//   - 圆形触发器：接近某个物体时触发
//   - 磁铁效果：吸引范围内的物品
//
// 技巧:
//   - 配合循环可以实现持续的范围监控
//   - 结合距离计算可以实现渐变效果
func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite
```

### 4. Tilemap 接口（5个瓦片地图方法）

```go
// PlaceTiles 批量放置瓦片
//
// 参数:
//   posXYs: 位置坐标数组，格式：[x1, y1, x2, y2, ...] - 所有要放置瓦片的坐标
//   texture: 瓦片纹理名称 - 对应资源文件中的图片名称
//   layer: 图层编号，默认为0 - 用于分层管理瓦片
//
// 特点:
//   - 高效的批量瓦片放置，比逐个调用PlaceTile性能更好
//   - 支持多图层系统，不同图层可以叠加显示
//   - 自动对齐到瓦片网格
//   - 覆盖已有瓦片时会替换原有内容
//
// 使用场景:
//   - 地图编辑器批量刷地形
//   - 程序化生成大片区域（森林、沙漠等）
//   - 关卡加载时批量放置背景瓦片
//   - 建筑系统一次性放置建筑组件
//
// 示例:
//   - 放置一排地面：PlaceTiles([]float64{0, 100, 32, 100, 64, 100}, "grass", 0)
//   - 批量放置背景：PlaceTiles(wallPositions, "brick_wall", 1)
//
// 注意事项:
//   - posXYs数组长度必须是偶数（成对的x,y坐标）
//   - 坐标会自动对齐到瓦片网格大小
//   - 相同位置的瓦片会被覆盖
func (p *Game) PlaceTiles(posXYs []float64, texture string, layer int64 = 0)
```

```go
// PlaceTile 放置单个瓦片
//
// 参数:
//   posX: 水平位置，单位：像素 - 会自动对齐到瓦片网格
//   posY: 垂直位置，单位：像素 - 会自动对齐到瓦片网格  
//   texture: 瓦片纹理名称 - 对应资源文件中的图片名称
//   layer: 图层编号，默认为0 - 用于分层管理瓦片
//
// 特点:
//   - 精确放置单个瓦片
//   - 支持多图层系统，高图层遮挡低图层
//   - 自动对齐到瓦片网格，确保瓦片整齐排列
//   - 实时生效，立即显示和参与碰撞检测
//
// 使用场景:
//   - 玩家手动建造系统
//   - 动态地形变化（挖掘、建造）
//   - 交互式地图编辑
//   - 实时放置装饰元素
//
// 示例:
//   - 放置地面瓦片：PlaceTile(100, 200, "grass", 0)
//   - 放置墙壁：PlaceTile(64, 64, "brick_wall", 1)
//   - 放置装饰：PlaceTile(150, 80, "flower", 2)
//
// 注意事项:
//   - 同一位置同一图层只能有一个瓦片
//   - 坐标会自动对齐，实际位置可能与输入略有差异
//   - 纹理名称必须在资源中存在，否则显示为空白
func (p *Game) PlaceTile(posX, posY float64, texture string, layer int64 = 0)
```

```go
// EraseTile 擦除瓦片
//
// 参数:
//   posX: 水平位置，单位：像素 - 要擦除的瓦片位置
//   posY: 垂直位置，单位：像素 - 要擦除的瓦片位置
//   layer: 图层编号，-1表示所有图层，其他值表示指定图层
//
// 特点:
//   - 移除指定位置的瓦片
//   - 支持按图层擦除或擦除所有图层
//   - 立即生效，瓦片消失并停止参与碰撞检测
//   - 坐标自动对齐到瓦片网格
//
// 使用场景:
//   - 挖掘系统（挖掉地形瓦片）
//   - 建筑拆除功能
//   - 地图编辑器的擦除工具
//   - 爆炸破坏效果
//
// 示例:
//   - 擦除指定图层：EraseTile(100, 200, 0)
//   - 擦除所有图层：EraseTile(100, 200, -1)
//   - 清理装饰层：EraseTile(64, 128, 2)
//
// 注意事项:
//   - layer=-1会擦除该位置所有图层的瓦片
//   - 擦除不存在的瓦片不会报错
//   - 坐标会自动对齐到瓦片网格
func (p *Game) EraseTile(posX, posY float64, layer int64 = -1)
```

```go
// GetTile 获取瓦片信息
//
// 参数:
//   posX: 水平位置，单位：像素 - 要查询的瓦片位置
//   posY: 垂直位置，单位：像素 - 要查询的瓦片位置
//   layer: 图层编号，-1表示查询最顶层，其他值表示指定图层
//
// 返回:
//   string: 瓦片纹理名称，如果该位置没有瓦片则返回空字符串
//
// 特点:
//   - 查询指定位置的瓦片信息
//   - 支持按图层查询或查询最顶层
//   - 坐标自动对齐到瓦片网格
//   - 不会修改瓦片状态，纯查询操作
//
// 使用场景:
//   - 碰撞检测前查询地形类型
//   - 建造系统检查位置是否已占用
//   - AI寻路时判断地形可通过性
//   - 地图编辑器显示当前选中瓦片
//
// 示例:
//   - 查询指定图层：tileName := GetTile(100, 200, 0)
//   - 查询最顶层：topTile := GetTile(64, 128, -1)
//   - 检查是否为空：if GetTile(x, y, 0) == "" { /* 位置为空 */ }
//
// 注意事项:
//   - layer=-1返回最高图层的瓦片（如果有多层）
//   - 空位置返回空字符串，不是nil
//   - 坐标会自动对齐到瓦片网格
func (p *Game) GetTile(posX, posY float64, layer int64 = -1) string
```

```


## API使用建议

### 1. 物理模式选择指南

```go
// DynamicPhysics - 适用于:
// - 玩家角色（需要重力和真实物理）
// - 可推动的物品（箱子、球等）
// - 受重力影响的敌人
player.SetPhysicsMode(DynamicPhysics)

// KinematicPhysics - 适用于:
// - 巡逻敌人（精确控制路径）
// - 移动平台（按预定轨迹移动）
// - 子弹和抛射物（不受重力影响）
// - 飞行敌人（不需要重力）
npc.SetPhysicsMode(KinematicPhysics)

// NoPhysics - 适用于:
// - 背景装饰（云朵、远山等）
// - 粒子效果
// - UI跟随元素
// - 不需要碰撞的视觉效果
decoration.SetPhysicsMode(NoPhysics)
```

### 2. 性能优化建议

```go
// 1. 合理使用NoPhysics模式减少计算开销
backgroundElements.SetPhysicsMode(NoPhysics)

// 2. 子弹等短生命周期对象使用KinematicPhysics
bullet.SetPhysicsMode(KinematicPhysics)  // 而不是DynamicPhysics

// 3. 静态物体，可以使用或StaticPhysics模式
backgroundElements.SetPhysicsMode(StaticPhysics)

```

### 3. 常见问题解决方案

```go
// 问题1: 角色卡在墙里
// 解决: 调整碰撞器大小，确保比显示图像略小
player.SetColliderRect(playerWidth * 0.8, playerHeight * 0.9)

// 问题2: 跳跃感觉不够responsive
// 解决: 使用AddImpulse而不是SetVelocity，并调整重力
if input.Jump && player.IsOnFloor() {
    player.AddImpulse(0, -350)  // 增大跳跃力
    player.SetGravity(1.2)  // 稍微增大重力，让跳跃更紧凑
}

// 问题3: 移动太滑
// 解决: 及时停止水平移动
if !input.Left && !input.Right {
    currentVel := player.Velocity()
    player.SetVelocity(0, currentVel.Y)  // 立即停止水平移动
}
```



## 用户故事与使用示例

**时间问题**，第一个用户故事 会使用`spx`代码来实现，其他的用户故事 用 `go` 代码实现

### 用户故事1: 制作巡逻敌人

**需求**: *"我需要一个在平台间来回巡逻的敌人，不受重力影响"*

PatrolEnemy.spx
```go
var (
    patrolPoints [][2]float64
    currentTarget int
    speed float64
)

func doUpdate(e *PatrolEnemy)  {
    currentPos := GetPosition()
    targetPos := patrolPoints[currentTarget]
    
    // 计算移动方向
    directionX := targetPos[0] - currentPos.X
    directionY := targetPos[1] - currentPos.Y
    
    // 检查是否到达目标点
    distance := math.Sqrt(float64(directionX*directionX + directionY*directionY))
    if distance < 10 {
        // 切换到下一个巡逻点
        currentTarget = (currentTarget + 1) % len(patrolPoints)
        return
    }
    
    // 标准化方向向量并设置速度
    length := float64(distance)
    if length > 0 {
        directionX /= length
        directionY /= length
        SetVelocity(directionX * speed, directionY * speed)
    }
}

onStart => {
    patrolPoints =  make([][2]float64, 2)
    patrolPoints[0] = [2]float64{100, 100}
    patrolPoints[1] = [2]float64{200, 100}
    currentTarget =  0,

    // 设置为运动学模式，不受重力影响
    SetPhysicsMode(KinematicPhysics)
    SetColliderRect(20, 20)
    setPosition(patrolPoints[0][0], patrolPoints[0][1])  // 从第一个点开始

    speed =  50,
	for {
		doUpdate(this)
		waitNextFrame()
	}
}

onTouchStart => {
	println("PatrolEnemy touched ...")
}

```

### 用户故事2: 制作平台跳跃游戏

**需求**: *"我要做一个马里奥风格的平台游戏，玩家可以左右移动和跳跃"*

```go
func setupPlayer() *SpriteImpl {
    player := NewSpriteImpl()
    
    // 设置为动态物理模式，受重力影响
    player.SetPhysicsMode(DynamicPhysics)
    player.SetGravity(1.0)  // 正常重力
    player.SetColliderRect(24, 32)  // 玩家碰撞器
    player.setPosition(100, 100)  // 初始位置
    
    return player
}

func updatePlayer(player *SpriteImpl, input GameInput) {
    currentVel := player.Velocity()
    
    // 左右移动
    var moveSpeed float64 = 200
    if input.Left {
        player.SetVelocity(-moveSpeed, currentVel.Y)
    } else if input.Right {
        player.SetVelocity(moveSpeed, currentVel.Y)
    } else {
        // 停止水平移动
        player.SetVelocity(0, currentVel.Y)
    }
    
    // 跳跃（只有在地面才能跳）
    if input.Jump && player.IsOnFloor() {
        player.AddImpulse(0, -300)  // 向上跳跃
    }
}
```

### 用户故事3: 制作射击游戏

**需求**: *"玩家可以射击，子弹要检测碰撞并消除敌人"*

```go
type Bullet struct {
    sprite *SpriteImpl
    damage int
    maxDistance float64
    startPosX, startPosY float64
}

func fireBullet(fromX, fromY, directionX, directionY, speed float64) *Bullet {
    bullet := &Bullet{
        sprite: NewSpriteImpl(),
        damage: 10,
        maxDistance: 500,
        startPosX: fromX,
        startPosY: fromY,
    }
    
    // 子弹使用运动学模式，不受重力影响
    bullet.sprite.SetPhysicsMode(KinematicPhysics)
    bullet.sprite.SetColliderRect(4, 4)  // 小碰撞器
    bullet.sprite.setPosition(fromX, fromY)
    
    // 设置子弹速度
    bullet.sprite.SetVelocity(
        directionX * speed,
        directionY * speed,
    )
    
    return bullet
}

func (b *Bullet) Update() {
    currentPos := b.sprite.GetPosition()
    
    // 检查飞行距离
    distance := math.Sqrt(float64(
        (currentPos.X-b.startPosX)*(currentPos.X-b.startPosX) +
        (currentPos.Y-b.startPosY)*(currentPos.Y-b.startPosY),
    ))
    
    if distance > float64(b.maxDistance) {
        b.Destroy()
        return
    }
    
    // 使用射线检测前方是否有敌人
    velocity := b.sprite.Velocity()
    nextPosX := currentPos.X + velocity.X * deltaTime // 假设16ms一帧
    nextPosY := currentPos.Y + velocity.Y * deltaTime
    
    hit, hitX, hitY, target := b.sprite.Raycast(currentPos.X, currentPos.Y, nextPosX, nextPosY)
    if hit && target != nil {
        // 击中目标
        if target.HasTag("enemy") {
            target.TakeDamage(b.damage)
            b.Destroy()
        }
    }
}
```

### 用户故事4: 制作可推动箱子

**需求**: *"场景中有箱子，玩家可以推动它们，箱子会受重力影响"*

```go
func createPushableBox(posX, posY float64) *SpriteImpl {
    box := NewSpriteImpl()
    
    // 箱子使用动态物理，可以被推动
    box.SetPhysicsMode(DynamicPhysics)
    box.SetGravity(1.0)  // 正常重力
    box.SetColliderRect(32, 32)
    box.setPosition(posX, posY)
    
    return box
}

func handlePlayerPushBox(player, box *SpriteImpl) {
    playerPos := player.GetPosition()
    boxPos := box.GetPosition()
    
    // 检查玩家是否贴着箱子
    distance := math.Abs(float64(playerPos.X - boxPos.X))
    if distance < 30 { // 在推动范围内
        // 计算推动方向
        pushDirection := float64(1)
        if playerPos.X > boxPos.X {
            pushDirection = -1  // 向左推
        }
        
        // 给箱子施加推力
        box.AddImpulse(pushDirection * 100, 0)
    }
}
```

### 用户故事5: 制作飞行道具效果

**需求**: *"玩家吃到飞行道具后可以飞行一段时间，不受重力影响"*

```go
type Player struct {
    sprite *SpriteImpl
    isFlying bool
    flyTimeLeft float64
}

func (p *Player) CollectFlyPowerup() {
    p.isFlying = true
    p.flyTimeLeft = 5.0  // 5秒飞行时间
    
    // 关闭重力
    p.sprite.SetGravity(0.0)
    
    // 给一个向上的推力
    p.sprite.AddImpulse(0, -200)
}

func (p *Player) UpdateFlying(deltaTime float64, input GameInput) {
    if !p.isFlying {
        return
    }
    
    // 飞行时间倒计时
    p.flyTimeLeft -= deltaTime
    if p.flyTimeLeft <= 0 {
        p.StopFlying()
        return
    }
    
    // 飞行控制
    var flySpeed float64 = 150
    var velocityX, velocityY float64 = 0, 0
    
    if input.Up {
        velocityY = -flySpeed
    } else if input.Down {
        velocityY = flySpeed
    }
    
    if input.Left {
        velocityX = -flySpeed
    } else if input.Right {
        velocityX = flySpeed
    }
    
    p.sprite.SetVelocity(velocityX, velocityY)
}

func (p *Player) StopFlying() {
    p.isFlying = false
    p.flyTimeLeft = 0
    
    // 恢复重力
    p.sprite.SetGravity(1.0)
}
```

### 用户故事6: 制作背景装饰元素

**需求**: *"背景中有飘动的云朵和飞鸟，它们不参与游戏碰撞"*

```go
type BackgroundCloud struct {
    sprite *SpriteImpl
    floatAmplitude float64
    floatSpeed float64
    startY float64
}

func createCloud(posX, posY float64) *BackgroundCloud {
    cloud := &BackgroundCloud{
        sprite: NewSpriteImpl(),
        floatAmplitude: 20,
        floatSpeed: 0.5,
        startY: posY,
    }
    
    // 云朵使用无物理模式，纯装饰
    cloud.sprite.SetPhysicsMode(NoPhysics)
    cloud.sprite.setPosition(posX, posY)
    // 不需要设置碰撞器，因为不参与碰撞
    
    return cloud
}

func (c *BackgroundCloud) Update(gameTime float64) {
    // 左右飘动
    currentPos := c.sprite.GetPosition()
    
    // 计算上下浮动
    floatY := c.startY + float64(math.Sin(float64(gameTime * c.floatSpeed))) * c.floatAmplitude
    
    // 水平移动
    newPosX := currentPos.X - 10 // 向左慢慢飘动
    newPosY := floatY
    
    // 直接设置位置（NoPhysics模式下推荐用setPosition而不是SetVelocity）
    c.sprite.setPosition(newPosX, newPosY)
    
    // 屏幕外循环
    if newPosX < -100 {
        c.sprite.setPosition(900, floatY) // 从右边重新出现
    }
}
```

### 用户故事7: 制作水下关卡

**需求**: *"某些关卡在水下，角色移动变慢，重力减小"*

```go
type WaterZone struct {
    bounds Rectangle  // 水域范围
    player *SpriteImpl
    inWater bool
    normalGravity float64
    normalSpeed float64
}

func (w *WaterZone) CheckPlayerInWater() {
    playerPos := w.player.GetPosition()
    
    wasInWater := w.inWater
    w.inWater = w.bounds.Contains(playerPos)
    
    // 状态改变时调整物理参数
    if w.inWater && !wasInWater {
        // 进入水中
        w.normalGravity = 1.0  // 记录正常重力
        w.normalSpeed = 200    // 记录正常速度
        
        w.player.SetGravity(0.3)  // 水中重力减小
    } else if !w.inWater && wasInWater {
        // 离开水中
        w.player.SetGravity(w.normalGravity)  // 恢复正常重力
    }
}

func (w *WaterZone) UpdatePlayerMovement(input GameInput) {
    speed := w.normalSpeed
    if w.inWater {
        speed *= 0.6  // 水中速度减慢
    }
    
    currentVel := w.player.Velocity()
    var newVelX float64 = 0
    
    if input.Left {
        newVelX = -speed
    } else if input.Right {
        newVelX = speed
    }
    
    // 水中可以向上游
    var newVelY float64 = currentVel.Y
    if w.inWater && input.Up {
        newVelY = -speed * 0.8  // 向上游的速度
    }
    
    w.player.SetVelocity(newVelX, newVelY)
}
```

### 用户故事8: 制作简单AI敌人

**需求**: *"敌人会追踪玩家，但被墙壁阻挡时会寻找绕路"*

```go
type ChaseEnemy struct {
    sprite *SpriteImpl
    target *SpriteImpl
    speed float64
    detectionRange float64
}

func createChaseEnemy(posX, posY float64, target *SpriteImpl) *ChaseEnemy {
    enemy := &ChaseEnemy{
        sprite: NewSpriteImpl(),
        target: target,
        speed: 80,
        detectionRange: 200,
    }
    
    // 敌人使用运动学模式，可以精确控制移动
    enemy.sprite.SetPhysicsMode(KinematicPhysics)
    enemy.sprite.SetGravity(0.0)  // 不受重力影响，可以飞行追击
    enemy.sprite.SetColliderRect(24, 24)
    enemy.sprite.setPosition(posX, posY)
    
    return enemy
}

func (e *ChaseEnemy) Update() {
    enemyPos := e.sprite.GetPosition()
    targetPos := e.target.GetPosition()
    
    // 检查距离
    dx := targetPos.X - enemyPos.X
    dy := targetPos.Y - enemyPos.Y
    distance := math.Sqrt(float64(dx*dx + dy*dy))
    
    // 超出检测范围则停止追击
    if distance > float64(e.detectionRange) {
        e.sprite.SetVelocity(0, 0)
        return
    }
    
    // 使用射线检测是否有障碍物
    hit, _, _, _ := e.sprite.Raycast(enemyPos.X, enemyPos.Y, targetPos.X, targetPos.Y)
    
    if !hit {
        // 没有障碍物，直接追击
        directionX := float64(dx) / float64(distance)
        directionY := float64(dy) / float64(distance)
        e.sprite.SetVelocity(
            directionX * e.speed,
            directionY * e.speed,
        )
    } else {
        // 有障碍物，尝试绕路（简单的左右尝试）
        // 尝试向左绕行
        leftPosX := enemyPos.X - 50
        leftPosY := targetPos.Y
        leftHit, _, _, _ := e.sprite.Raycast(enemyPos.X, enemyPos.Y, leftPosX, leftPosY)
        
        if !leftHit {
            e.sprite.SetVelocity(-e.speed, 0)
        } else {
            // 尝试向右绕行
            rightPosX := enemyPos.X + 50
            rightPosY := targetPos.Y
            rightHit, _, _, _ := e.sprite.Raycast(enemyPos.X, enemyPos.Y, rightPosX, rightPosY)
            
            if !rightHit {
                e.sprite.SetVelocity(e.speed, 0)
            } else {
                // 都被挡住了，停止移动
                e.sprite.SetVelocity(0, 0)
            }
        }
    }
}
```

### 用户故事9: 制作扫雷游戏

**需求**: *"我要做一个经典的扫雷游戏，点击翻开格子，右键标记地雷，简单易懂的俯视角游戏"*

```go
type MinesweeperGame struct {
    game *Game
    mapWidth, mapHeight int
    tileSize int
    mineCount int
    gameState string // "playing", "won", "lost"
    minePositions map[string]bool // 存储地雷位置
    revealedCount int
}

func createMinesweeperGame(game *Game, width, height, mines int) *MinesweeperGame {
    mg := &MinesweeperGame{
        game: game,
        mapWidth: width,
        mapHeight: height,
        tileSize: 32,
        mineCount: mines,
        gameState: "playing",
        minePositions: make(map[string]bool),
        revealedCount: 0,
    }
    
    // 初始化游戏
    mg.initializeGame()
    
    return mg
}

func (mg *MinesweeperGame) initializeGame() {
    // 批量生成隐藏的格子
    hiddenPositions := make([]float64, 0)
    for x := 0; x < mg.mapWidth; x++ {
        for y := 0; y < mg.mapHeight; y++ {
            hiddenPositions = append(hiddenPositions, 
                float64(x*mg.tileSize), float64(y*mg.tileSize))
        }
    }
    
    // 放置隐藏格子 (layer 0 - 所有格子都是隐藏状态)
    mg.game.PlaceTiles(hiddenPositions, "hidden", 0)
    
    // 随机生成地雷位置
    mg.generateMines()
}

func (mg *MinesweeperGame) generateMines() {
    minesPlaced := 0
    for minesPlaced < mg.mineCount {
        // 随机选择位置
        x := rand.Intn(mg.mapWidth)
        y := rand.Intn(mg.mapHeight)
        
        key := fmt.Sprintf("%d,%d", x, y)
        
        // 如果该位置还没有地雷，放置地雷
        if !mg.minePositions[key] {
            mg.minePositions[key] = true
            minesPlaced++
        }
    }
}

func (mg *MinesweeperGame) handlePlayerInput(input GameInput) {
    if mg.gameState != "playing" {
        return // 游戏已结束
    }
    
    mouseX, mouseY := input.GetMouseWorldPos()
    
    // 转换为网格坐标
    gridX := int(mouseX / float64(mg.tileSize))
    gridY := int(mouseY / float64(mg.tileSize))
    
    // 检查边界
    if gridX < 0 || gridX >= mg.mapWidth || gridY < 0 || gridY >= mg.mapHeight {
        return
    }
    
    tileX := float64(gridX * mg.tileSize)
    tileY := float64(gridY * mg.tileSize)
    
    if input.LeftClick {
        // 左键翻开格子
        mg.revealTile(gridX, gridY, tileX, tileY)
    } else if input.RightClick {
        // 右键标记/取消标记
        mg.toggleFlag(gridX, gridY, tileX, tileY)
    }
}

func (mg *MinesweeperGame) revealTile(gridX, gridY int, tileX, tileY float64) {
    // 检查当前格子状态
    currentTile := mg.game.GetTile(tileX, tileY, 0)
    
    // 如果已经翻开或被标记，不能翻开
    if currentTile != "hidden" {
        return
    }
    
    key := fmt.Sprintf("%d,%d", gridX, gridY)
    
    // 检查是否是地雷
    if mg.minePositions[key] {
        // 踩到地雷 - 游戏失败
        mg.game.PlaceTile(tileX, tileY, "mine", 0)
        mg.gameState = "lost"
        mg.revealAllMines()
        fmt.Println("游戏失败！踩到地雷了！")
        return
    }
    
    // 计算周围地雷数量
    mineCount := mg.countAdjacentMines(gridX, gridY)
    
    if mineCount == 0 {
        // 周围无地雷，显示空格并自动展开
        mg.game.PlaceTile(tileX, tileY, "empty", 0)
        mg.revealedCount++
        
        // 自动展开相邻的空格子
        mg.autoRevealAdjacent(gridX, gridY)
    } else {
        // 显示数字
        numberTexture := fmt.Sprintf("number_%d", mineCount)
        mg.game.PlaceTile(tileX, tileY, numberTexture, 0)
        mg.revealedCount++
    }
    
    // 检查是否获胜
    mg.checkWinCondition()
}

func (mg *MinesweeperGame) toggleFlag(gridX, gridY int, tileX, tileY float64) {
    currentTile := mg.game.GetTile(tileX, tileY, 0)
    
    if currentTile == "hidden" {
        // 标记为地雷
        mg.game.PlaceTile(tileX, tileY, "flag", 0)
    } else if currentTile == "flag" {
        // 取消标记
        mg.game.PlaceTile(tileX, tileY, "hidden", 0)
    }
    // 已翻开的格子不能标记
}

func (mg *MinesweeperGame) countAdjacentMines(centerX, centerY int) int {
    count := 0
    
    // 检查周围8个方向
    for dx := -1; dx <= 1; dx++ {
        for dy := -1; dy <= 1; dy++ {
            if dx == 0 && dy == 0 {
                continue // 跳过中心点
            }
            
            x := centerX + dx
            y := centerY + dy
            
            // 检查边界
            if x >= 0 && x < mg.mapWidth && y >= 0 && y < mg.mapHeight {
                key := fmt.Sprintf("%d,%d", x, y)
                if mg.minePositions[key] {
                    count++
                }
            }
        }
    }
    
    return count
}

func (mg *MinesweeperGame) autoRevealAdjacent(centerX, centerY int) {
    // 自动翻开相邻的格子（仅当中心格子为空时）
    for dx := -1; dx <= 1; dx++ {
        for dy := -1; dy <= 1; dy++ {
            if dx == 0 && dy == 0 {
                continue
            }
            
            x := centerX + dx
            y := centerY + dy
            
            // 检查边界
            if x >= 0 && x < mg.mapWidth && y >= 0 && y < mg.mapHeight {
                tileX := float64(x * mg.tileSize)
                tileY := float64(y * mg.tileSize)
                
                currentTile := mg.game.GetTile(tileX, tileY, 0)
                
                // 只翻开隐藏的格子
                if currentTile == "hidden" {
                    mg.revealTile(x, y, tileX, tileY)
                }
            }
        }
    }
}

func (mg *MinesweeperGame) revealAllMines() {
    // 游戏失败时显示所有地雷
    for key := range mg.minePositions {
        coords := strings.Split(key, ",")
        x, _ := strconv.Atoi(coords[0])
        y, _ := strconv.Atoi(coords[1])
        
        tileX := float64(x * mg.tileSize)
        tileY := float64(y * mg.tileSize)
        
        currentTile := mg.game.GetTile(tileX, tileY, 0)
        
        // 只显示未标记的地雷
        if currentTile == "hidden" {
            mg.game.PlaceTile(tileX, tileY, "mine", 0)
        }
    }
}

func (mg *MinesweeperGame) checkWinCondition() {
    totalTiles := mg.mapWidth * mg.mapHeight
    tilesWithoutMines := totalTiles - mg.mineCount
    
    if mg.revealedCount == tilesWithoutMines {
        mg.gameState = "won"
        fmt.Println("恭喜！你赢了！")
        
        // 自动标记所有地雷
        mg.markAllMines()
    }
}

func (mg *MinesweeperGame) markAllMines() {
    // 获胜时自动标记所有地雷
    for key := range mg.minePositions {
        coords := strings.Split(key, ",")
        x, _ := strconv.Atoi(coords[0])
        y, _ := strconv.Atoi(coords[1])
        
        tileX := float64(x * mg.tileSize)
        tileY := float64(y * mg.tileSize)
        
        mg.game.PlaceTile(tileX, tileY, "flag", 0)
    }
}

func (mg *MinesweeperGame) restartGame() {
    // 重置游戏状态
    mg.gameState = "playing"
    mg.minePositions = make(map[string]bool)
    mg.revealedCount = 0
    
    // 清空所有瓦片
    for x := 0; x < mg.mapWidth; x++ {
        for y := 0; y < mg.mapHeight; y++ {
            tileX := float64(x * mg.tileSize)
            tileY := float64(y * mg.tileSize)
            mg.game.EraseTile(tileX, tileY, 0)
        }
    }
    
    // 重新初始化
    mg.initializeGame()
    fmt.Println("游戏重新开始！")
}

// 主更新函数
func (mg *MinesweeperGame) Update(input GameInput) {
    // 处理玩家输入
    mg.handlePlayerInput(input)
    
    // 检查重新开始（按R键）
    if input.KeyPressed("R") {
        mg.restartGame()
    }
}

// 使用示例
func main() {
    game := NewGame()
    
    // 创建一个 10x10 的扫雷游戏，包含15个地雷
    minesweeper := createMinesweeperGame(game, 10, 10, 15)
    
    // 游戏循环
    for {
        input := game.GetInput()
        minesweeper.Update(input)
        
        game.Render()
        game.WaitNextFrame()
    }
}
```