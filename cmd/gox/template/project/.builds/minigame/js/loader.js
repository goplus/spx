import "./libs/godot";
import { GodotSDK } from "./libs/sdk"

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

const crypto = {
  getRandomValues: (view) => {
    for (let i = 0; i < view.length; i++) {
      // Math.random() 生成一个 0 到 1 之间的浮动值，将其乘以 256，取整并限制在 0-255 之间
      view[i] = Math.floor(Math.random() * 256);
    }
    return view;
  },
};

class FakeBlob {
  constructor(data, options) {
    this.data = data || [];
    this.type = options?.type || "";
    this.size = this.data.reduce((total, item) => total + (item.length || 0), 0);
  }
}
const godotSdk = new GodotSDK()
GameGlobal.WebAssembly = WXWebAssembly;
GameGlobal.crypto = crypto;
// 整个假的Blob，websocket防止出错
GameGlobal.Blob = FakeBlob;
GameGlobal.godotSdk = godotSdk;



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
    // 创建着色器
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
    // Step 4: 创建顶点缓冲区并绑定
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

  load() {
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
      .then(() => {
        this.updateLoading();
        const engine = new Engine();
        GameGlobal.engine = engine;
        godotSdk.set_engine(engine);
        return engine.startGame({
          canvas: canvas,
          executable: "engine/godot",
          mainPack: "engine/godot.zip",
          args: ["--audio-driver", "ScriptProcessor"],
        });
      })
      .then(() => {
        engine.config.persistentPaths.forEach(path => {
          godotSdk.copyLocalToFS(path);
        })
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
      });
  }
}

export default Loader;