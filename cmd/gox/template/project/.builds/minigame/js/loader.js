import "weapp-adapter";
import "fetch";
import GameRunner from "./runner";
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

console.log("crypto", crypto)
console.log("globalThis.crypto", globalThis.crypto)

// Use dynamic import to avoid hoisting issues
let GodotSDK;
async function loadModules() {
  await import("./fetch.js"); // Load fetch polyfill first
  await import("./engine");
  const sdkModule = await import("./sdk");
  GodotSDK = sdkModule.GodotSDK;
}

const LoaderConfig = {
  logo: "images/logo.png",
  background: "images/background.png",
  iconWidth: 128,
  iconHeight: 128,
  backGroudColor: "#282c34",
  loadingBarHeight: 20,
  loadingBarColor: "#478CBF",
  loadingBarBackgroundColor: "#444",
};

class FakeBlob {
  constructor(data, options) {
    this.data = data || [];
    this.type = options?.type || "";
    this.size = this.data.reduce((total, item) => total + (item.length || 0), 0);
  }
}
let godotSdk;

// Initialization function
async function initializeSDK() {
  await loadModules();
  godotSdk = new GodotSDK();
  GameGlobal.WebAssembly = WXWebAssembly;
  GameGlobal.crypto = crypto;
  // Fake Blob for websocket error prevention
  GameGlobal.Blob = FakeBlob;
  GameGlobal.godotSdk = godotSdk;
}



class Loader {
  constructor(config) {
    this.config = {
      ...LoaderConfig,
      ...config,
    };
    const info = wx.getWindowInfo();
    const dpr = info.pixelRatio;
    this.progress = 0;
    this.screenContext = canvas.getContext("webgl2");
    this.loadingCanvas = document.createElement("canvas");
    this.loadingContext = this.loadingCanvas.getContext("2d");
    this.loadingCanvas.width = window.innerWidth * dpr;
    this.loadingCanvas.height = window.innerHeight * dpr;
    canvas.width = window.innerWidth * dpr;
    canvas.height = window.innerHeight * dpr;
    this.loadingContext.scale(dpr, dpr);

    this.backgroundImage = wx.createImage();
    this.backgroundImage.src = this.config.background;

    this.logoImage = wx.createImage();
    this.logoImage.src = this.config.logo;
    this.logoImage.width = this.config.iconWidth;
    this.logoImage.height = this.config.iconHeight;

    const [screenTexture, cleanWebgl] = this.initWebgl();
    this.screenTexture = screenTexture;
    this.cleanWebgl = cleanWebgl;
  }

  loadSubpackages() {
    return new Promise((resolve, reject) => {
      wx.loadSubpackage({
        fail: (reason) => {
          reject(reason);
        },
        name: "engine",
        success: () => {
          this.updateLoading();
          resolve();
        },
      });
    });
  }

  drawLoadingBar() {
    const barWidth = window.innerWidth - 48;
    const barX = (window.innerWidth - barWidth) / 2;
    const barY = window.innerHeight - this.config.loadingBarHeight / 2 - 100;
    const ctx = this.loadingContext;

    // Draw background of loading bar
    ctx.fillStyle = this.config.loadingBarBackgroundColor;
    ctx.fillRect(barX, barY, barWidth, this.config.loadingBarHeight);

    // Draw the progress
    ctx.fillStyle = this.config.loadingBarColor;
    ctx.fillRect(
      barX,
      barY,
      (this.progress / 3) * barWidth,
      this.config.loadingBarHeight
    );

    // Add text percentage
    ctx.font = "16px";
    ctx.fillStyle = "#fff";
    ctx.textAlign = "center";
    ctx.fillText(
      `${((this.progress / 3) * 100).toFixed(1)}%`,
      window.innerWidth / 2,
      barY + this.config.loadingBarHeight - 6
    );
  }

  drawBackground() {
    const ctx = this.loadingContext;
    const canvasWidth = this.loadingCanvas.width;
    const canvasHeight = this.loadingCanvas.height;
    const imageAspectRatio =
      this.backgroundImage.naturalWidth / this.backgroundImage.naturalWidth;
    const canvasAspectRatio = canvasWidth / canvasHeight;
    let drawWidth, drawHeight, offsetX, offsetY;
    if (canvasAspectRatio > imageAspectRatio) {
      // Canvas is wider than the image, fit by height.
      drawHeight = canvasHeight;
      drawWidth = drawHeight * imageAspectRatio;
      offsetX = (canvasWidth - drawWidth) / 2;
      offsetY = 0;
    } else {
      // Canvas is taller than the image, fit by width.
      drawWidth = canvasWidth;
      drawHeight = drawWidth / imageAspectRatio;
      offsetX = 0;
      offsetY = (canvasHeight - drawHeight) / 2;
    }
    ctx.drawImage(
      this.backgroundImage,
      offsetX,
      offsetY,
      drawWidth,
      drawHeight
    );
  }

  drawIcon() {
    const ctx = this.loadingContext;
    const centerX = window.innerWidth / 2 - this.config.iconWidth / 2;
    const centerY = window.innerHeight / 3 - this.config.iconHeight / 3;
    ctx.drawImage(
      this.logoImage,
      centerX,
      centerY,
      this.config.iconWidth,
      this.config.iconHeight
    );
  }
  onProgress(value) {
    this.progress = value;
    console.log("====>onProgress", value)
  }

  updateLoading() {
    this.progress += 1;
    if (this.progress > 3) this.progress = 3;
    this.drawBackground();
    this.drawIcon();
    this.drawLoadingBar();
    this.drawScreen();
  }

  initWebgl() {
    const gl = this.screenContext;
    gl.bindTexture(gl.TEXTURE_2D, texture);
    // Create shaders
    const vertexShaderSource = `
      attribute vec4 a_position;
      attribute vec2 a_texCoord;
      varying vec2 v_texCoord;
      void main() {
        gl_Position = a_position;
        v_texCoord = a_texCoord;
      }
    `;

    const fragmentShaderSource = `
      precision mediump float;
      varying vec2 v_texCoord;
      uniform sampler2D u_texture;
      void main() {
        gl_FragColor = texture2D(u_texture, v_texCoord);
      }
    `;

    function createShader(type, source) {
      const shader = gl.createShader(type);
      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
        console.error("Error compiling shader:", gl.getShaderInfoLog(shader));
        gl.deleteShader(shader);
        return null;
      }
      return shader;
    }
    const vertexShader = createShader(gl.VERTEX_SHADER, vertexShaderSource);
    const fragmentShader = createShader(
      gl.FRAGMENT_SHADER,
      fragmentShaderSource
    );
    const shaderProgram = gl.createProgram();
    gl.attachShader(shaderProgram, vertexShader);
    gl.attachShader(shaderProgram, fragmentShader);
    gl.linkProgram(shaderProgram);
    if (!gl.getProgramParameter(shaderProgram, gl.LINK_STATUS)) {
      console.error(
        "Program linking error:",
        gl.getProgramInfoLog(shaderProgram)
      );
    }
    gl.useProgram(shaderProgram);
    // Step 4: Create vertex buffer and bind
    const vertices = new Float32Array([
      -1.0, 1.0, 0.0, 0.0, -1.0, -1.0, 0.0, 1.0, 1.0, -1.0, 1.0, 1.0, -1.0, 1.0,
      0.0, 0.0, 1.0, -1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.0,
    ]);
    const vertexBuffer = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, vertexBuffer);
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);
    const positionLocation = gl.getAttribLocation(shaderProgram, "a_position");
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 16, 0);
    gl.enableVertexAttribArray(positionLocation);

    const texCoordLocation = gl.getAttribLocation(shaderProgram, "a_texCoord");
    gl.vertexAttribPointer(texCoordLocation, 2, gl.FLOAT, false, 16, 8);
    gl.enableVertexAttribArray(texCoordLocation);

    const texture = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.viewport(0, 0, this.loadingCanvas.width, this.loadingCanvas.height);
    const clean = () => {
      const maxAttributes = gl.getParameter(gl.MAX_VERTEX_ATTRIBS);
      for (let i = 0; i < maxAttributes; i++) {
        gl.disableVertexAttribArray(i);
      }
      if (texture) gl.deleteTexture(texture);
      gl.bindTexture(gl.TEXTURE_2D, null);
      if (vertexShader) gl.deleteShader(vertexShader);
      if (fragmentShader) gl.deleteShader(fragmentShader);
      if (shaderProgram) gl.deleteProgram(shaderProgram);
      gl.bindBuffer(gl.ARRAY_BUFFER, null);
      gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, null);
      gl.bindTexture(gl.TEXTURE_2D, null);
      gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      gl.bindRenderbuffer(gl.RENDERBUFFER, null);
      gl.clear(
        gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT
      );
      gl.viewport(0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight);
      gl.disable(gl.DEPTH_TEST);
      gl.disable(gl.BLEND);
      gl.disable(gl.CULL_FACE);
    };
    return [texture, clean];
  }

  drawScreen() {
    const gl = this.screenContext;
    gl.bindTexture(gl.TEXTURE_2D, this.screenTexture);
    gl.texImage2D(
      gl.TEXTURE_2D,
      0,
      gl.RGBA,
      gl.RGBA,
      gl.UNSIGNED_BYTE,
      this.loadingCanvas
    );
    gl.clearColor(0.0, 0.0, 0.0, 1.0);
    gl.clear(gl.COLOR_BUFFER_BIT);

    gl.drawArrays(gl.TRIANGLES, 0, 6);
  }

  clean() {
    this.logoImage = null;
    this.loadingContext.clearRect(
      0,
      0,
      this.loadingCanvas.width,
      this.loadingCanvas.height
    );
    const gl = this.screenContext;
  }

  async load() {
    // Initialize SDK first
    await initializeSDK();

    const loadLogo = () => {
      return new Promise((resolve, reject) => {
        this.logoImage.onload = () => {
          resolve();
        };
        this.logoImage.onerror = (error) => {
          reject(error);
        };
      });
    };
    const loadBackground = () => {
      return new Promise((resolve, reject) => {
        this.backgroundImage.onload = () => {
          resolve();
        };
        this.backgroundImage.error = (error) => {
          reject(error);
        };
      });
    };
    Promise.all([loadBackground(), loadLogo()])
      .then(() => {
        this.progress += 1;
        return this.loadSubpackages();
      })
      .then(async () => {
        this.updateLoading();
        const runner = new GameRunner(this);
          await runner.startGame(this.onStart.bind(this),this.onProgress.bind(this));
      });
  }
  async onStart() {
    console.log("====>onStart")
    // engine.config.persistentPaths.forEach(path => {
    //   godotSdk.copyLocalToFS(path);
    // })
    godotSdk.syncfs(() => {
    }, (error) => {
      console.error(error)
    });
    setInterval(() => {
      godotSdk.syncfs(() => {
      }, (error) => {
        console.error(error)
      });
    }, 5000)
    this.clean();
    this.cleanWebgl();
  }


}

export default Loader;