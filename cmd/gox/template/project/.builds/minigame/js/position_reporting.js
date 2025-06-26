let position = 0
let ended = false
let lastPostTime = 0
let currentTime = 0

worker.onMessage(event => {

  if (event.type === "ended") {
    position = 0;
    ended = false;
    lastPostTime = 0;
    currentTime = 0;
    return
  }

  if (event.type === 'init') {
    currentTime = event.currentTime;
  }

  if (event.type === "process") {

    position += event.inputLength
    if (event.currentTime - lastPostTime > 0.1) {
      lastPostTime = event.currentTime;
      worker.postMessage({type: 'position', data: position})
    }
  }
})
