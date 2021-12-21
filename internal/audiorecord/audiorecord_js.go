package audiorecord

import (
	"github.com/goplus/spx/internal/coroutine"
	"syscall/js"
)

const scriptCode = `!function(t,e){"object"==typeof exports&&"undefined"!=typeof module?e(exports):"function"==typeof define&&define.amd?define(["exports"],e):e((t||self).GopAudioRecorder={})}(this,function(t){var e,i=/*#__PURE__*/function(){function t(t){this.audioContext=t,this.connectingToMic=!1,this.mic=null}var e=t.prototype;return e.getLoudness=function(){var t=this;if(this.mic||this.connectingToMic||(this.connectingToMic=!0,navigator.mediaDevices.getUserMedia({audio:!0}).then(function(e){t.audioStream=e,t.mic=t.audioContext.createMediaStreamSource(e),t.analyser=t.audioContext.createAnalyser(),t.mic.connect(t.analyser),t.micDataArray=new Float32Array(t.analyser.fftSize)}).catch(function(t){console.warn(t)})),this.mic&&this.audioStream&&this.audioStream.active){this.analyser.getFloatTimeDomainData(this.micDataArray);for(var e=0,i=0;i<this.micDataArray.length;i++)e+=Math.pow(this.micDataArray[i],2);var a=Math.sqrt(e/this.micDataArray.length);return this._lastValue&&(a=Math.max(a,.6*this._lastValue)),this._lastValue=a,a*=1.63,a=Math.sqrt(a),a=Math.round(100*a),(a=Math.min(a,100))/100}return 0},e.release=function(){this.connectingToMic=!1,this.mic&&(this.mic.disconnect(),this.mic=null),this.audioStream&&(this.audioStream.getTracks().forEach(function(t){return t.stop()}),this.audioStream=void 0),this.analyser&&(this.analyser.disconnect(),this.analyser=void 0)},t}(),a=new(window.AudioContext||window.webkitAudioContext)({latencyHint:"interactive"}),n=new i(a);t.start=function(t){try{return clearInterval(e),a.resume(),e=setInterval(function(){t(n.getLoudness())},100),Promise.resolve()}catch(t){return Promise.reject(t)}},t.stop=function(){clearInterval(e),n.release()}});`

var scriptInited bool

func initScript() {
	if scriptInited {
		return
	}
	scriptInited = true

	window := js.Global().Get("window")
	document := js.Global().Get("document")

	// docuemnt is undefined on node.js
	if !document.Truthy() {
		return
	}

	if !document.Get("body").Truthy() {
		ch := make(chan struct{})
		window.Call("addEventListener", "load", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			close(ch)
			return nil
		}))
		<-ch
	}

	script := js.Global().Get("document").Call("createElement", "script")
	script.Set("innerHTML", scriptCode)
	js.Global().Get("document").Get("body").Call("appendChild", script)
}

type Recorder struct {
	volume float64
}

func Open(gco *coroutine.Coroutines) *Recorder {
	p := &Recorder{}
	initScript()
	audioRecorder := js.Global().Get("GopAudioRecorder")
	audioRecorder.Call("start", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		volume := args[0].Float()
		p.volume = volume
		return nil
	}))
	return p
}

func (p *Recorder) Close() {
	audioRecorder := js.Global().Get("GopAudioRecorder")
	audioRecorder.Call("stop", nil)
}

func (p *Recorder) Loudness() float64 {
	return p.volume
}
