class GodotSDK {
  set_engine(engine) {
    this.engine = engine;
  }

  writeFile(path, array) {
    const fs = wx.getFileSystemManager();
    const idx = path.lastIndexOf("/");
    let dir = "/";
    if (idx > 0) {
      dir = path.slice(0, idx);
    }

    // 确保目录存在
    return new Promise((resolve, reject) => {
      fs.access({
        path: `${wx.env.USER_DATA_PATH}${dir}`,
        success: resolve,
        fail: () => {
          fs.mkdir({
            dirPath: `${wx.env.USER_DATA_PATH}${dir}`,
            recursive: true,
            success: resolve,
            fail: resolve
          });
        }
      });
    }).then(() => {
      // 打开文件
      return new Promise((resolve, reject) => {
        fs.open({
          filePath: `${wx.env.USER_DATA_PATH}${path}`,
          flag: "w+",
          success: (res) => resolve(res.fd),
          fail: reject
        });
      });
    }).then((fd) => {
      // 写入数据
      return new Promise((resolve, reject) => {
        fs.write({
          fd: fd,
          data: array.buffer,
          success: () => resolve(fd),
          fail: (error) => reject({
            fd,
            error
          })
        });
      });
    }).then((fd) => {
      // 关闭文件
      return new Promise((resolve, reject) => {
        fs.close({
          fd: fd,
          success: resolve,
          fail: reject
        });
      });
    }).catch((error) => {
      if (error.fd !== undefined) {
        return new Promise((resolve, reject) => {
          fs.close({
            fd: error.fd,
            success: resolve,
            fail: reject
          });
        }).then(() => {
          throw error.error;
        });
      } else {
        throw error;
      }
    });
  }

  copyLocalToFS(path) {
    const fs = wx.getFileSystemManager();

    // 检查路径是否存在
    return new Promise((resolve, reject) => {
      fs.access({
        path: `${wx.env.USER_DATA_PATH}${path}`,
        success: resolve,
        fail: reject
      });
    }).catch(() => {
      console.warn(`Path does not exist: ${path}`);
      return Promise.resolve();
    }).then(() => {
      // 读取目录内容
      return new Promise((resolve, reject) => {
        fs.readdir({
          dirPath: `${wx.env.USER_DATA_PATH}${path}`,
          success: (res) => resolve(res.files.filter(v => v !== "." && v !== "..")),
          fail: reject
        });
      });
    }).then((dirs) => {
      // 处理每个文件或子目录
      return dirs.reduce((promiseChain, dir) => {
        return promiseChain.then(() => {
          const p = `${wx.env.USER_DATA_PATH}${path}/${dir}`;

          return new Promise((resolve, reject) => {
            fs.stat({
              path: p,
              success: res => resolve(res.stats),
              fail: reject
            });
          }).then((stat) => {
            if (stat.isDirectory()) {
              return this.copyLocalToFS(`${path}/${dir}`);
            } else if (stat.isFile()) {
              return new Promise((resolve, reject) => {
                fs.readFile({
                  filePath: p,
                  success: (res) => {
                    this.engine.copyToFS(`${path}/${dir}`, res.data);
                    resolve();
                  },
                  fail: reject
                });
              });
            }
          });
        });
      }, Promise.resolve());
    });
  }

  syncfs(onSuccess, onError) {
    this.engine.copyFSToAdapter(this).then(() => {
      onSuccess()
    }).catch((error) => {
      onError(error)
    });
  }
}

export {
  GodotSDK
};