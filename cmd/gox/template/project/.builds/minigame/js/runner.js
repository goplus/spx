class GameRunner {
  constructor(loader) {
    this.loader = loader;
  }

  async startGame(onStart,onProgress) {
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
      "onStart": onStart ,
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
    return
  }
}

export default GameRunner;
