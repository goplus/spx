import "./adpter";
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
    for (const [path, entry] of Object.entries(unzipped)) {
        if (path.endsWith('/')) continue;
        const content = (entry.byteOffset === 0 && entry.byteLength === entry.buffer.byteLength)
            ? entry.buffer
            : entry.slice().buffer;
        files[path] = { lastModified: now, content };
    }
    return files;
}

class MiniGameRunner {
    constructor() {
        this.godotSdk = new GodotSDK();
        GameGlobal.godotSdk = this.godotSdk;
        this.gameRunner = null;
        this.syncfsInterval = null;
    }

    async onGameStart() {
        console.log('Game started')
        const syncFiles = () => this.godotSdk.syncfs(() => {}, error => console.error(error));
        syncFiles();
        if (this.syncfsInterval != null) {
            clearInterval(this.syncfsInterval);
        }
        this.syncfsInterval = setInterval(syncFiles, 5000)
    }

    async startGame(onProgress) {
        const response = await fetch("engine/game.zip");
        if (!response.ok) {
            throw new Error(`Failed to fetch engine/game.zip: HTTP ${response.status}`);
        }
        const zipped = await response.arrayBuffer();
        const files = buildFilesFromZip(zipped);
        const config = {
            projectName: "spx_game",
            onProgress,
            gameCanvas: canvas,
            logLevel: 0,
            isRuntimeMode: true,
            assetURLs: {
                "engine.zip": "engine/engine.zip",
                "game.zip": "engine/game.zip",
                "ispx.wasm": "engine/ispx.wasm",
                "engine.wasm": "engine/engine.wasm",
            },
        };
        if (this.gameRunner != null) {
            await this.gameRunner.ResetGame();
        }

        this.gameRunner = new globalThis.GameRunner(config);
        await this.gameRunner.InitEngine();
        await this.gameRunner.InitGame(files);
        await this.gameRunner.StartGame();
        await this.onGameStart();
    }
}

export default MiniGameRunner;
