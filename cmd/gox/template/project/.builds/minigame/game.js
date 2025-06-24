import "./weapp-adapter";
import "./fetch";
import Loader from "./loader";

if (!globalThis.TextEncoder) {
  globalThis.TextEncoder = class {
    encode(string) {
      const utf8 = [];
      for (let i = 0; i < string.length; i++) {
        let charcode = string.charCodeAt(i);
        if (charcode < 0x80) utf8.push(charcode);
        else if (charcode < 0x800) {
          utf8.push(0xc0 | (charcode >> 6), 0x80 | (charcode & 0x3f));
        }
        else if (charcode < 0xd800 || charcode >= 0xe000) {
          utf8.push(0xe0 | (charcode >> 12), 0x80 | ((charcode>>6) & 0x3f), 0x80 | (charcode & 0x3f));
        }
        else {
          i++;
          charcode = 0x10000 + (((charcode & 0x3ff)<<10) | (string.charCodeAt(i) & 0x3ff));
          utf8.push(0xf0 | (charcode >>18), 0x80 | ((charcode>>12) & 0x3f), 0x80 | ((charcode>>6) & 0x3f), 0x80 | (charcode & 0x3f));
        }
      }
      return new Uint8Array(utf8);
    }
  };
}

if (!globalThis.TextDecoder) {
  globalThis.TextDecoder = class {
    decode(bytes) {
      let string = "";
      let i = 0;
      while (i < bytes.length) {
        let c = bytes[i++];
        if (c > 127) {
          if (c > 191 && c < 224) {
            c = (c & 31) << 6 | bytes[i++] & 63;
          } else if (c > 223 && c < 240) {
            c = (c & 15) << 12 | (bytes[i++] & 63) << 6 | bytes[i++] & 63;
          } else if (c > 239 && c < 248) {
            c = (c & 7) << 18 | (bytes[i++] & 63) << 12 | (bytes[i++] & 63) << 6 | bytes[i++] & 63;
          }
        }
        string += String.fromCharCode(c);
      }
      return string;
    }
  };
}

function checkUpdate() {
  const updateManager = wx.getUpdateManager();
  updateManager.onCheckForUpdate(() => {
    // 请求完新版本信息的回调
    // console.log(res.hasUpdate)
  });
  updateManager.onUpdateReady(() => {
    wx.showModal({
      title: "更新提示",
      content: "新版本已经准备好，是否重启应用？",
      success(res) {
        if (res.confirm) {
          // 新的版本已经下载好，调用 applyUpdate 应用新版本并重启
          updateManager.applyUpdate();
        }
      },
    });
  });
  updateManager.onUpdateFailed(() => {
    // 新版本下载失败
  });
}
checkUpdate();
const loader = new Loader();
loader.load();
