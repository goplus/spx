package audiorecord

import (
	"strings"
	"syscall/js"
)

var (
	window   = js.Global().Get("window")
	document = js.Global().Get("document")
	volume   float64
)

var scriptCode = strings.Trim(`
!function(e,t){"object"==typeof exports&&"undefined"!=typeof module?t(exports):"function"==typeof define&&define.amd?define(["exports"],t):t((e||self).QNAudioRecorder={})}(this,function(e){var t=new(window.AudioContext||window.webkitAudioContext)({latencyHint:"interactive"}),n=t.createAnalyser();n.fftSize=2048;var o=t.createGain();o.gain.value=0,n.connect(o),o.connect(t.destination);var i,r,a,c=new Uint8Array(n.frequencyBinCount);function s(){i&&(i.getTracks().forEach(function(e){return e.stop()}),i=void 0),r&&(r.disconnect(),r=void 0),clearInterval(a),t.suspend()}e.start=function(e){try{return s(),Promise.resolve(navigator.mediaDevices.getUserMedia({audio:!0})).then(function(o){(r=t.createMediaStreamSource(i=o)).connect(n),t.resume(),a=setInterval(function(){var o,i;n.getByteFrequencyData(c),e((o=0,i=c.length,c.forEach(function(e,n){var r=n*(t.sampleRate||44100)/i;if(r>22050)i-=1;else{var a,c,s=187374169.94399998*(c=(a=r)*a)*c/((c+424.36)*Math.sqrt((c+11599.29)*(c+544496.41))*(c+14884e4))*e/255;s<=0?i-=1:o+=s*s}}),0===i?0:Math.sqrt(o/i)))},100)})}catch(e){return Promise.reject(e)}},e.stop=function(){s()}});
`, "\n\t ")

func init() {
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

func StartRecorder() {
	qnAudioRecorder := js.Global().Get("QNAudioRecorder")
	qnAudioRecorder.Call("start", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		volume = args[0].Float()
		return nil
	}))
}

func StopRecorder() {
	qnAudioRecorder := js.Global().Get("QNAudioRecorder")
	qnAudioRecorder.Call("stop", nil)
}

func Loudness() float64 {
	return volume
}
