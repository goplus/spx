# Godot 与 SPX 游戏暂停系统详解

## 概述

本文档详细描述了 Godot 引擎的暂停机制以及 SPX 引擎如何基于 Godot 实现游戏暂停和恢复功能。

---

## 第一部分：Godot 引擎的暂停机制

### 1. 核心暂停控制

Godot 通过 `SceneTree` 类提供全局暂停功能：

```cpp
// 核心接口 (scene/main/scene_tree.h)
class SceneTree {
    bool paused = false;  // 全局暂停状态标志
    
public:
    void set_pause(bool p_enabled);  // 设置暂停状态
    bool is_paused() const;          // 查询暂停状态
};
```

### 2. 暂停实现机制

#### 2.1 暂停设置流程

```cpp
// scene/main/scene_tree.cpp:880-892
void SceneTree::set_pause(bool p_enabled) {
    ERR_FAIL_COND_MSG(!Thread::is_main_thread(), "Pause can only be set from the main thread.");
    
    if (p_enabled == paused) {
        return;  // 状态未改变
    }
    
    paused = p_enabled;
    
    // 1. 停止/恢复物理服务器
    PhysicsServer3D::get_singleton()->set_active(!p_enabled);
    PhysicsServer2D::get_singleton()->set_active(!p_enabled);
    
    // 2. 向所有节点传播暂停通知
    if (get_root()) {
        get_root()->_propagate_pause_notification(p_enabled);
    }
}
```

#### 2.2 节点处理模式 (ProcessMode)

每个节点都有一个处理模式，决定在暂停时的行为：

```cpp
// scene/main/node.h:69-74
enum ProcessMode {
    PROCESS_MODE_INHERIT,      // 继承父节点模式（默认）
    PROCESS_MODE_PAUSABLE,     // 可暂停模式
    PROCESS_MODE_WHEN_PAUSED,  // 仅暂停时处理
    PROCESS_MODE_ALWAYS,       // 始终处理
    PROCESS_MODE_DISABLED      // 禁用处理
};
```

#### 2.3 暂停判断核心逻辑

```cpp
// scene/main/node.cpp:771-798
bool Node::_can_process(bool p_paused) const {
    ProcessMode process_mode = data.process_mode;
    
    // 继承模式处理
    if (process_mode == PROCESS_MODE_INHERIT) {
        process_mode = data.process_owner ? 
            data.process_owner->data.process_mode : PROCESS_MODE_PAUSABLE;
    }
    
    // 关键判断逻辑
    switch (process_mode) {
        case PROCESS_MODE_DISABLED:
            return false;
        case PROCESS_MODE_ALWAYS:
            return true;
        case PROCESS_MODE_WHEN_PAUSED:
            return p_paused;      // 仅暂停时处理
        case PROCESS_MODE_PAUSABLE:
        default:
            return !p_paused;     // 仅非暂停时处理
    }
}
```

#### 2.4 暂停通知传播

```cpp
// scene/main/node.cpp:594-609
void Node::_propagate_pause_notification(bool p_enable) {
    bool prev_can_process = _can_process(!p_enable);
    bool next_can_process = _can_process(p_enable);
    
    // 发送状态变化通知
    if (prev_can_process && !next_can_process) {
        notification(NOTIFICATION_PAUSED);
    } else if (!prev_can_process && next_can_process) {
        notification(NOTIFICATION_UNPAUSED);
    }
    
    // 递归传播给所有子节点
    data.blocked++;
    for (auto& child : data.children) {
        child.value->_propagate_pause_notification(p_enable);
    }
    data.blocked--;
}
```

### 3. 各系统的暂停响应

#### 3.1 动画系统
- **AnimationPlayer**: 通过停止内部处理循环暂停，状态保持
- **Tween**: 根据 `TweenPauseMode` 决定行为

#### 3.2 物理系统
- **2D/3D 物理**: 服务器直接停用，所有刚体停止运动
- 即使设置为 `PROCESS_MODE_ALWAYS` 也不会处理物理

#### 3.3 音频系统
```cpp
// scene/audio/audio_stream_player.cpp:78-87
case NOTIFICATION_PAUSED: {
    if (!can_process()) {
        set_stream_paused(true);  // 暂停音频流
    }
} break;

case NOTIFICATION_UNPAUSED: {
    set_stream_paused(false);     // 恢复音频流
} break;
```

#### 3.4 粒子系统
```cpp
// scene/2d/gpu_particles_2d.cpp:699-708
case NOTIFICATION_PAUSED:
case NOTIFICATION_UNPAUSED: {
    if (is_inside_tree()) {
        if (can_process()) {
            RS::get_singleton()->particles_set_speed_scale(particles, speed_scale);
        } else {
            RS::get_singleton()->particles_set_speed_scale(particles, 0);  // 速度设为0
        }
    }
} break;
```

---

## 第二部分：SPX 引擎的暂停实现

### 1. SPX 暂停架构设计

SPX 引擎采用分层架构，通过监听 Godot 的暂停通知来实现与 Godot 的同步暂停：

```
外部调用层: SpxExtMgr
    ↓
入口管理层: Spx
    ↓  
核心逻辑层: SpxEngine
    ↓
Godot 系统: SceneTree
    ↓ (暂停通知)
监听节点层: SpxEngineNode
    ↓ (状态同步)
管理器层: SpxBaseMgr 子类们
```

### 2. 核心组件实现

#### 2.1 监听节点 (SpxEngineNode)

```cpp
// core/extension/spx.cpp
class SpxEngineNode : public Node {
    GDCLASS(SpxEngineNode, Node);
    
protected:
    void _notification(int p_what) override {
        switch (p_what) {
            case NOTIFICATION_PAUSED:
                if (SpxEngine::has_initialed()) {
                    SpxEngine::get_singleton()->_on_godot_pause_changed(true);
                }
                break;
                
            case NOTIFICATION_UNPAUSED:
                if (SpxEngine::has_initialed()) {
                    SpxEngine::get_singleton()->_on_godot_pause_changed(false);
                }
                break;
        }
    }
    
public:
    SpxEngineNode() {
        // 设置为 ALWAYS 以确保能接收暂停通知
        set_process_mode(PROCESS_MODE_ALWAYS);
    }
};
```

#### 2.2 核心引擎 (SpxEngine)

```cpp
// core/extension/spx_engine.h
class SpxEngine : SpxBaseMgr {
private:
    bool is_spx_paused = false;
    
public:
    // 公共暂停接口
    void pause();   // 设置 Godot SceneTree 暂停
    void resume();  // 取消 Godot SceneTree 暂停
    bool is_paused() const;
    
    // 内部同步方法（由 SpxEngineNode 调用）
    void _on_godot_pause_changed(bool is_godot_paused);
};
```

```cpp
// core/extension/spx_engine.cpp
void SpxEngine::pause() {
    if (tree != nullptr) {
        tree->set_pause(true);  // 只设置 Godot 状态
    }
    // SPX 状态将通过 SpxEngineNode 的通知机制自动同步
}

void SpxEngine::resume() {
    if (tree != nullptr) {
        tree->set_pause(false);
    }
}

void SpxEngine::_on_godot_pause_changed(bool is_godot_paused) {
    if (is_godot_paused && !is_spx_paused) {
        // Godot 暂停，SPX 同步暂停
        is_spx_paused = true;
        for (auto mgr : mgrs) {
            mgr->on_pause();
        }
    } else if (!is_godot_paused && is_spx_paused) {
        // Godot 恢复，SPX 同步恢复
        is_spx_paused = false;
        for (auto mgr : mgrs) {
            mgr->on_resume();
        }
    }
}
```

#### 2.3 入口层接口 (Spx)

```cpp
// core/extension/spx.h
class Spx {
public:
    static void pause();        // 暂停游戏
    static void resume();       // 恢复游戏
    static bool is_paused();    // 查询暂停状态
};
```

```cpp
// core/extension/spx.cpp
void Spx::pause() {
    if (initialed && SpxEngine::has_initialed()) {
        SPX_ENGINE->pause();  // 委托给 SpxEngine
    }
}

void Spx::resume() {
    if (initialed && SpxEngine::has_initialed()) {
        SPX_ENGINE->resume();
    }
}

bool Spx::is_paused() {
    if (initialed && SpxEngine::has_initialed()) {
        return SPX_ENGINE->is_paused();
    }
    return false;
}
```

#### 2.4 扩展管理器接口 (SpxExtMgr)

```cpp
// core/extension/spx_ext_mgr.h
class SpxExtMgr : SpxBaseMgr {
public:
    void pause();           // 外部调用接口
    void resume();
    GdBool is_paused();
};
```

```cpp
// core/extension/spx_ext_mgr.cpp  
void SpxExtMgr::pause() {
    Spx::pause();          // 委托给上层 Spx 接口
}

void SpxExtMgr::resume() {
    Spx::resume();
}

GdBool SpxExtMgr::is_paused() {
    return Spx::is_paused();
}
```

#### 2.5 管理器基类支持 (SpxBaseMgr)

```cpp
// core/extension/spx_base_mgr.h
class SpxBaseMgr {
public:
    virtual void on_pause() {}   // 暂停时回调
    virtual void on_resume() {}  // 恢复时回调
};
```

### 3. SPX 暂停工作流程

#### 3.1 用户手动暂停流程

```
用户调用 SpxExtMgr::pause()
    ↓
Spx::pause()
    ↓  
SpxEngine::pause()
    ↓
SceneTree::set_pause(true)  [设置 Godot 暂停]
    ↓
Godot 发送 NOTIFICATION_PAUSED 到所有节点
    ↓
SpxEngineNode 接收通知
    ↓
SpxEngine::_on_godot_pause_changed(true)
    ↓
设置 is_spx_paused = true + 通知所有 SPX 管理器暂停
```

#### 3.2 编辑器暂停按钮流程

```
用户点击 Godot 编辑器暂停按钮
    ↓
SceneTree::set_pause(true)  [Godot 内部调用]
    ↓
Godot 发送 NOTIFICATION_PAUSED 到所有节点  
    ↓
SpxEngineNode 接收通知
    ↓
SpxEngine::_on_godot_pause_changed(true)
    ↓
设置 is_spx_paused = true + 通知所有 SPX 管理器暂停
```

#### 3.3 子管理器暂停处理

```cpp
// 示例：音频管理器
class SpxAudioMgr : public SpxBaseMgr {
    Vector<AudioStreamPlayer*> active_players;
    
public:
    void on_pause() override {
        for (auto player : active_players) {
            if (player && player->is_playing()) {
                player->set_stream_paused(true);
            }
        }
    }
    
    void on_resume() override {
        for (auto player : active_players) {
            if (player) {
                player->set_stream_paused(false);
            }
        }
    }
};
```

### 4. SPX 暂停特性

#### 4.1 统一的暂停入口
- 无论是代码调用还是编辑器操作，都通过 Godot 的 SceneTree 暂停机制
- 避免了多个暂停路径导致的状态不一致

#### 4.2 自动状态同步
- SPX 自动跟随 Godot 的暂停状态变化
- 无需手动维护两套暂停状态

#### 4.3 管理器级暂停支持
- 所有 SPX 子管理器都可以实现自定义的暂停/恢复逻辑
- 提供统一的暂停事件通知机制

#### 4.4 架构清晰
- 分层设计，职责明确
- 外部接口简洁，内部实现灵活

