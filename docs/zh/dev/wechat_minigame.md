## 微信小游戏实现

### 目标
让spx2 支持导出成微信小游戏

### 问题
1. 单线程限制  (web 独立worker模式 将不能导出，需要兼容两种模式)
2. 微信小程序包体限制 (关键时候需要进行分包下载)
3. godot 当前版本音频解决方案 依赖于 AudioWorklet，微信小游戏不支持

### 解决方案
1. 兼容 独立worker 和 单线程模式
2. 音频方案
 - 替换godot实现，不依赖于 AudioWorklet
 - 封装微信接口
3. 文件系统
 - 封装微信接口

### 开发计划

### 实现细节
1. 替换微信API
 - 移除 canvas 的获取: this.canvas = /** @type {!HTMLCanvasElement} */ (first);
2. wasm 加载机制
 - wasm 加载方式默认改为 .br
3. 文件系统：
 - 增加接口 copyFSToAdapter 用于拷贝数据到文件系统
 - 移除 module['initFS'](paths)
 - Preloader : wx.getFileSystemManager
4. 音频模块
 - const createAudioContext = wx.createWebAudioContext;



### 参考资料
1. [微信小游戏](https://developers.weixin.qq.com/minigame/dev/guide/index.html)
2. [godot](https://godotengine.org/)
3. [godot-wechat](https://github.com/yuchenyang1994/godot-love-wechat)

