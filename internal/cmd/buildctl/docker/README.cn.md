## 使用说明

> **Legacy / 不再支持：** 本目录保留的是历史 Podman 构建实验，Dockerfile
> 内的 EMSDK、NDK 等版本不会跟随 `internal/release/runtime.lock.json`。
> 当前 SPX/Godot 构建请使用仓库根目录的 `make dev`、`make build-*`，或
> 对应的 `buildctl build` 命令；不要将这里的镜像用于正式 runtime 发布。

### 0. 准备
1. 安装 [Podman](https://podman.io/), 如果你使用的是 Ubuntu, 可以使用 `sudo apt install podman` 来安装.
2. 准备好vpn
3. 构建基础镜像，或拉取已构建的基础镜像

### 1. 构建基础镜像 
```
./build_containers.sh <vpn_proxy_url>
# eg: ./build_containers.sh http://192.168.31.147:7890
```
