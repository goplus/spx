
import FakeBlob from "./adpter";
import GodotSDK from "./sdk";
import "./engine";
import "./fflate";

function buildFilesFromZip(data) {
    if (globalThis.fflate == null || typeof globalThis.fflate.unzipSync !== "function") {
        throw new Error("fflate.unzipSync is unavailable");
    }

    const files = {};
    const now = Date.now();
    const unzipped = globalThis.fflate.unzipSync(new Uint8Array(data));
    Object.entries(unzipped).forEach(([path, entry]) => {
        if (path.endsWith('/')) return;
        const content = (entry.byteOffset === 0 && entry.byteLength === entry.buffer.byteLength)
            ? entry.buffer
            : entry.slice().buffer;
        files[path] = { lastModified: now, content };
    });
    return files;
}

class GameRunner {
    constructor() {
        this.godotSdk = new GodotSDK();
        GameGlobal.godotSdk = this.godotSdk;
        this.gameApp = null;
        this.syncfsInterval = null;
    }
    async onGameStart() {
        console.log("====>onStart")
        this.godotSdk.syncfs(() => {
        }, (error) => {
            console.error(error)
        });
        if (this.syncfsInterval != null) {
            clearInterval(this.syncfsInterval);
        }
        this.syncfsInterval = setInterval(() => {
            this.godotSdk.syncfs(() => {
            }, (error) => {
                console.error(error)
            });
        }, 5000)
    }

    async startGame(onProgress) {
        const response = await fetch("engine/game.zip");
        if (!response.ok) {
            throw new Error(`Failed to fetch engine/game.zip: HTTP ${response.status}`);
        }
        const zipped = await response.arrayBuffer();
        const files = buildFilesFromZip(zipped);
        let assetURLs = null
        const config = {
            'projectName': "spx_game",
            'onProgress': onProgress,
            "gameCanvas": canvas,
            "logLevel": 0,
            "isRuntimeMode": true,
            "assetURLs": {
                "engine.zip": "engine/engine.zip",
                "game.zip": "engine/game.zip",
                "ispx.wasm": "engine/ispx.wasm",
                "engine.wasm": "engine/engine.wasm",
            },
        };
        if (assetURLs != null) {
            config.assetURLs = assetURLs
        }

        if (this.gameApp != null) {
            await this.gameApp.ResetGame();
        }

        this.gameApp = new GameApp(config);
        await this.gameApp.InitEngine();
        await this.gameApp.InitGame(files);
        await this.gameApp.StartGame();
        await this.onGameStart();
    }
}

export default GameRunner;
