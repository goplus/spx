## 使用说明
### 0. 准备
1. 安装 [Podman](https://podman.io/), 如果你使用的是 Ubuntu, 可以使用 `sudo apt install podman` 来安装.
2. 准备好vpn
3. 构建基础镜像，或拉取已构建的基础镜像

### 1. 构建基础镜像 
```
./build_containers.sh <vpn_proxy_url>
# eg: ./build_containers.sh http://192.168.31.147:7890
```
