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

    // Ensure directory exists
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
      // Open file
      return new Promise((resolve, reject) => {
        fs.open({
          filePath: `${wx.env.USER_DATA_PATH}${path}`,
          flag: "w+",
          success: (res) => resolve(res.fd),
          fail: reject
        });
      });
    }).then((fd) => {
      // Write data
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
      // Close file
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

    // Check if path exists
    return new Promise((resolve, reject) => {
      fs.access({
        path: `${wx.env.USER_DATA_PATH}${path}`,
        success: resolve,
        fail: reject
      });
    }).catch(() => {
      console.warn(`Path does not exist: ${path}`);
      return new Promise((reslove, reject) => {
        fs.mkdir({
          dirPath: `${wx.env.USER_DATA_PATH}${path}`,
          recursive: true,
          success: () => {
            reslove()
          },
          fail: reject
        })
      })
    }).then(() => {
      // Read directory content
      return new Promise((resolve, reject) => {
        fs.readdir({
          dirPath: `${wx.env.USER_DATA_PATH}${path}`,
          success: (res) => resolve(res.files.filter(v => v !== "." && v !== "..")),
          fail: reject
        });
      });
    }).then((dirs) => {
      // Process each file or subdirectory
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
      if (onSuccess) {
        onSuccess()
      }
    }).catch((error) => {
      if (onError) {
        onError(error)
      }
    });
  }

  downloadSubpcks(onSuccess, onError) {
    return new Promise((resolve, reject) => {
      wx.loadSubpackage({
        fail: reject,
        name: 'subpacks',
        success: () => resolve(),
      })
    }).then(() => {
      const fs = wx.getFileSystemManager();
      return new Promise((resolve, reject) => {
        fs.readdir({
          dirPath: "subpacks",
          success: res => {
            resolve(res.files);
          },
          fail: reject
        })
      })
    }).then(files => {
      const promises = files.filter(file => !file.endsWith(".js")).map(file => {
        return new Promise((resolve, reject) => {
          const fs = wx.getFileSystemManager()
          fs.readFile({
            filePath: `subpacks/${file}`,
            success: (res) => resolve({name: file, data: res.data}),
            fail: reject
          })
        })
      })
      return Promise.all(promises)
    }).then(values => {
      values.map(value => {
        const path = `subpacks/${value.name}`;
        this.engine.copyToFS(path, value.data);
      })
      onSuccess()
    }).catch(reason => {
      if (onError) {
        onError(reason.errMsg);
      }
    })
  }

  downloadCDNSubpcks(url, onSucess, onError) {
    return new Promise((resolve, reject) => {
      wx.request({
        url: url,
        responseType: "arraybuffer",
        method: "GET",
        success: res => resolve(res.data),
        fail: reject
      })
    }).then(data => {
      const filename = url.split('/').pop();
      this.engine.copyToFS(`subpacks/${filename}`, data)
      if (onSucess) {
        onSucess();
      }
    }).catch(reason => {
      if (onError) {
        onError(reason);
      }
    })
  }
}


export default GodotSDK