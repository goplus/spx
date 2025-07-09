import Loader from "./js/loader";


function checkUpdate() {
  const updateManager = wx.getUpdateManager();
  updateManager.onCheckForUpdate(() => {
    // Callback for when new version info request is completed
    // console.log(res.hasUpdate)
  });
  updateManager.onUpdateReady(() => {
    wx.showModal({
      title: "Update Notice",
      content: "New version is ready, restart application?",
      success(res) {
        if (res.confirm) {
          // New version downloaded, call applyUpdate to apply new version and restart
          updateManager.applyUpdate();
        }
      },
    });
  });
  updateManager.onUpdateFailed(() => {
    // New version download failed
  });
}
checkUpdate();
const loader = new Loader();
loader.load();
