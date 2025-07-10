
import FakeBlob from "./adpter";
import GodotSDK from "./sdk";
import "./engine";

class GameRunner {
    constructor() {
        this.godotSdk = new GodotSDK();
        GameGlobal.godotSdk = this.godotSdk;
    }
    async onGameStart() {
        console.log("====>onStart")
        this.godotSdk.syncfs(() => {
        }, (error) => {
            console.error(error)
        });
        setInterval(() => {
            this.godotSdk.syncfs(() => {
            }, (error) => {
                console.error(error)
            });
        }, 5000)
    }

    async startGame(onProgress) {
        // Use fetch polyfill to get files
        let buffer = await (await fetch("engine/game.zip")).arrayBuffer();
        let assetURLs = null
        const config = {
            'projectName': "spx_game",
            'onProgress': onProgress,
            "gameCanvas": canvas,
            "editorCanvas": canvas,
            "projectData": new Uint8Array(buffer),
            "logLevel": 0,
            "onStart": () => {
                this.onGameStart()
            },
            "useAssetCache": false,
            "isRuntimeMode": true,
            "assetURLs": {
                "engine.zip": "engine/engine.zip",
                "game.zip": "engine/game.zip",
                "gdspx.wasm": "engine/gdspx.wasm",
                "engine.wasm": "engine/engine.wasm",
            },
        };
        if (assetURLs != null) {
            config.assetURLs = assetURLs
        }

        let gameApp = new GameApp(config);
        await gameApp.RunGame();
    }
}

export default GameRunner;
