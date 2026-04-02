import "./weapp-adapter";
import "./fetch";

// First set crypto polyfill, must be before importing Godot editor
const crypto = {
  getRandomValues: (view) => {
    for (let i = 0; i < view.length; i++) {
      // Math.random() generates a float value between 0 and 1, multiply by 256, round and limit to 0-255
      view[i] = Math.floor(Math.random() * 256);
    }
    return view;
  },
};

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
// Immediately set to globalThis to ensure Godot editor can find it when importing
globalThis.crypto = crypto;
// Use dynamic import to avoid hoisting issues

class FakeBlob {
  constructor(data, options) {
    this.data = data || [];
    this.type = options?.type || "";
    this.size = this.data.reduce((total, item) => total + (item.length || 0), 0);
  }
}

GameGlobal.WebAssembly = WXWebAssembly;
GameGlobal.crypto = crypto;
// Fake Blob for websocket error prevention
GameGlobal.Blob = FakeBlob;

export default FakeBlob