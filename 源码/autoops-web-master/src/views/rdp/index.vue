<template>
  <div class="rdp-client">
    <!-- 连接覆盖层 -->
    <div v-if="overlayVisible" class="overlay">
      <div v-if="overlayLoading" class="spinner"></div>
      <span class="overlay-text">{{ overlayText }}</span>
    </div>

    <!-- 麦克风指示灯 -->
    <div v-if="micActive" class="mic-indicator"></div>

    <!-- 渲染画布 -->
    <canvas
      ref="canvasRef"
      :style="canvasStyle"
      @mousemove="handleMouseMove"
      @mousedown="handleMouseDown"
      @mouseup="handleMouseUp"
      @wheel="handleWheel"
      @contextmenu.prevent
    ></canvas>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import API from '@/api/cmdb'
import storage from '@/utils/storage'
import {
  buildRdpWsUrl,
  extractRDPSession,
  clearCachedRDPSession
} from '@/utils/rdpSession'

// Props
const props = defineProps({
  // URL 参数
  w: { type: Number, default: null },
  h: { type: Number, default: null },
  rdpid: { type: String, default: '' },
  sessionId: { type: String, default: '' },
  id: { type: String, default: '' },
  monitor: { type: String, default: null },
  primary: { type: String, default: null },
  nodpr: { type: Boolean, default: false },
  interpolate: { type: Boolean, default: false },
  scale: { type: String, default: '' },
  devscale: { type: String, default: '' },
  abuf: { type: Number, default: 15 },
  kb: { type: String, default: 'scancode' },
  // WebSocket URL
  wsUrl: { type: String, default: '' }
  ,
  // 新增：显示缩放比例，默认 0.75 即缩小到 75%
  displayScale: { type: Number, default:0.8 }
})

// Emits
const emit = defineEmits(['connected', 'disconnected', 'error'])

// Refs
const canvasRef = ref(null)

// 状态
const overlayVisible = ref(true)
const overlayLoading = ref(true)
const overlayText = ref('Connecting...')
const micActive = ref(false)

// 尺寸相关
const dpr = ref(1)
const W = ref(0)
const H = ref(0)

// WebSocket
const ws = ref(null)
const activeSession = ref(null)

// 渲染相关
const gl = ref(null)
const glTex = ref(null)
const ctx = ref(null)
const glNeedsRedraw = ref(false)
const glFilter = ref(0)
const h264Canvas = ref(null)
const h264Ctx = ref(null)

// 光标缓存
const cursorCache = ref({})
const cursorCanvas = ref(null)
const cursorCtx = ref(null)

// 音频相关
const audioCtx = ref(null)
const audioNode = ref(null)
const audioUseWorklet = ref(false)
const audioRingL = ref(null)
const audioRingR = ref(null)
const audioRingSize = 262144
const audioRingMask = audioRingSize - 1
const audioWritePos = ref(0)
const audioReadPos = ref(0)
const audioResumed = ref(false)
const audioPreBuffered = ref(false)
const audioBufMs = ref(props.abuf)
const audioPreBufThreshold = ref(0)

// 麦克风相关
const micStream = ref(null)
const micAudioCtx = ref(null)
const micSource = ref(null)
const micProcessor = ref(null)
const micBrowserRate = ref(0)
const micAccumBufs = ref(null)
const micAccumFrames = ref(0)
const micAccumTarget = ref(0)

// 摄像头相关
const camEncoder = ref(null)
const camStream = ref(null)
const camReader = ref(null)
const camRunning = ref(false)

// H.264 解码器
const h264Decoder = ref(null)
const h264LumaDecoder = ref(null)
const h264Timestamp = ref(0)
const h264RectMap = ref({})
const h264GotKeyframe = ref(false)
const h264LumaGotKeyframe = ref(false)

// 剪贴板
const remoteClipText = ref('')

// 鼠标状态
const mouseButtons = ref(0)

// 键盘映射
const isMac = ref(/Mac|iPhone|iPad/.test(navigator.platform))

// 防抖定时器
let resizeTimer = null

// 计算属性
const canvasStyle = computed(() => {
  if (!W.value || !H.value || !dpr.value) return {}
  const baseWidth = W.value / dpr.value
  const baseHeight = H.value / dpr.value
  return {
    width: `${baseWidth * props.displayScale}px`,
    height: `${baseHeight * props.displayScale}px`,
    cursor: 'default',
    display: 'block',
    margin: 'auto'
  }
})

const getUrlParam = (name) => {
  const params = new URLSearchParams(window.location.search)
  return params.get(name)
}

const sessionIdFromQuery = computed(() => {
  return props.sessionId || getUrlParam('sessionId') || ''
})

const rdpID = computed(() => {
  // 优先从 props 获取，其次从 URL 获取，最后默认 '1'
  return props.rdpid || props.id || getUrlParam('rdpid') || getUrlParam('id') || ''
})

// 键盘扫描码映射
const scancodeMap = {
  Escape: 0x01, Digit1: 0x02, Digit2: 0x03, Digit3: 0x04, Digit4: 0x05,
  Digit5: 0x06, Digit6: 0x07, Digit7: 0x08, Digit8: 0x09, Digit9: 0x0a,
  Digit0: 0x0b, Minus: 0x0c, Equal: 0x0d, Backspace: 0x0e, Tab: 0x0f,
  KeyQ: 0x10, KeyW: 0x11, KeyE: 0x12, KeyR: 0x13, KeyT: 0x14, KeyY: 0x15,
  KeyU: 0x16, KeyI: 0x17, KeyO: 0x18, KeyP: 0x19, BracketLeft: 0x1a,
  BracketRight: 0x1b, Enter: 0x1c, ControlLeft: 0x1d, KeyA: 0x1e,
  KeyS: 0x1f, KeyD: 0x20, KeyF: 0x21, KeyG: 0x22, KeyH: 0x23, KeyJ: 0x24,
  KeyK: 0x25, KeyL: 0x26, Semicolon: 0x27, Quote: 0x28, Backquote: 0x29,
  ShiftLeft: 0x2a, Backslash: 0x2b, KeyZ: 0x2c, KeyX: 0x2d, KeyC: 0x2e,
  KeyV: 0x2f, KeyB: 0x30, KeyN: 0x31, KeyM: 0x32, Comma: 0x33, Period: 0x34,
  Slash: 0x35, ShiftRight: 0x36, NumpadMultiply: 0x37, AltLeft: 0x38,
  Space: 0x39, CapsLock: 0x3a, F1: 0x3b, F2: 0x3c, F3: 0x3d, F4: 0x3e,
  F5: 0x3f, F6: 0x40, F7: 0x41, F8: 0x42, F9: 0x43, F10: 0x44,
  NumLock: 0x45, ScrollLock: 0x46, Numpad7: 0x47, Numpad8: 0x48,
  Numpad9: 0x49, NumpadSubtract: 0x4a, Numpad4: 0x4b, Numpad5: 0x4c,
  Numpad6: 0x4d, NumpadAdd: 0x4e, Numpad1: 0x4f, Numpad2: 0x50,
  Numpad3: 0x51, Numpad0: 0x52, NumpadDecimal: 0x53, F12: 0x58,
  NumpadEnter: 0x1c, ControlRight: 0x1d, NumpadDivide: 0x35,
  PrintScreen: 0x37, AltRight: 0x38, Home: 0x47, ArrowUp: 0x48,
  PageUp: 0x49, ArrowLeft: 0x4b, ArrowRight: 0x4d, End: 0x4f,
  ArrowDown: 0x50, PageDown: 0x51, Insert: 0x52, Delete: 0x53,
  MetaLeft: 0x5b, MetaRight: 0x5c, ContextMenu: 0x5d
}

const extendedKeys = new Set([
  'NumpadEnter', 'ControlRight', 'NumpadDivide', 'PrintScreen', 'AltRight',
  'Home', 'ArrowUp', 'PageUp', 'ArrowLeft', 'ArrowRight', 'End', 'ArrowDown',
  'PageDown', 'Insert', 'Delete', 'MetaLeft', 'MetaRight', 'ContextMenu'
])

const controlKeyCodes = new Set([
  'Escape', 'Backspace', 'Tab', 'Enter', 'NumpadEnter', 'ControlLeft',
  'ControlRight', 'ShiftLeft', 'ShiftRight', 'AltLeft', 'AltRight',
  'MetaLeft', 'MetaRight', 'CapsLock', 'NumLock', 'ScrollLock', 'F1', 'F2',
  'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F12', 'PrintScreen',
  'Insert', 'Delete', 'Home', 'End', 'PageUp', 'PageDown', 'ArrowUp',
  'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ContextMenu'
])

// 音频 Worklet 代码
const audioWorkletCode = `
class AudioOutputProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.ringL = new Float32Array(262144);
    this.ringR = new Float32Array(262144);
    this.mask = 262143;
    this.wp = 0;
    this.rp = 0;
    this.preBuffered = false;
    this.port.onmessage = (e) => {
      var left = e.data.left;
      var right = e.data.right;
      var frames = left.length;
      var wp = this.wp;
      for (var i = 0; i < frames; i++) {
        var idx = wp & this.mask;
        this.ringL[idx] = left[i];
        this.ringR[idx] = right[i];
        wp++;
      }
      this.wp = wp & this.mask;
      var avail = (this.wp - this.rp) & this.mask;
      if (avail > 253952) {
        this.rp = (this.wp - 22050) & this.mask;
      }
    };
  }
  process(inputs, outputs) {
    var outL = outputs[0][0];
    var outR = outputs[0][1];
    var len = outL.length;
    var avail = (this.wp - this.rp) & this.mask;
    if (!this.preBuffered) {
      if (avail < 8192) {
        for (var i = 0; i < len; i++) { outL[i] = 0; outR[i] = 0; }
        return true;
      }
      this.preBuffered = true;
    }
    var count = avail < len ? avail : len;
    var rp = this.rp;
    for (var i = 0; i < count; i++) {
      var idx = rp & this.mask;
      outL[i] = this.ringL[idx];
      outR[i] = this.ringR[idx];
      rp++;
    }
    for (var i = count; i < len; i++) { outL[i] = 0; outR[i] = 0; }
    this.rp = rp & this.mask;
    if (count < len) { this.preBuffered = false; }
    return true;
  }
}
registerProcessor('audio-output-processor', AudioOutputProcessor);
`

// 初始化渲染器
function initRenderer() {
  const canvas = canvasRef.value
  if (!canvas) return

  canvas.width = W.value
  canvas.height = H.value

  let webgl = canvas.getContext('webgl2', { antialias: false, alpha: false })
  if (!webgl) {
    webgl = canvas.getContext('webgl', { antialias: false, alpha: false })
  }

  if (!webgl) {
    ctx.value = canvas.getContext('2d')
    if (!props.interpolate) {
      canvas.style.imageRendering = 'pixelated'
    }
    console.log('canvas: using Canvas 2D fallback')
    ctx.value.fillStyle = '#000'
    ctx.value.fillRect(0, 0, W.value, H.value)
    return
  }

  gl.value = webgl
  console.log('canvas: using WebGL (' + gl.value.getParameter(gl.value.VERSION) + ')')

  const vs = gl.value.createShader(gl.value.VERTEX_SHADER)
  gl.value.shaderSource(vs,
    'attribute vec2 a_pos;' +
    'varying vec2 v_uv;' +
    'void main(){v_uv=a_pos*0.5+0.5;v_uv.y=1.0-v_uv.y;gl_Position=vec4(a_pos,0.0,1.0);}'
  )
  gl.value.compileShader(vs)

  const fs = gl.value.createShader(gl.value.FRAGMENT_SHADER)
  gl.value.shaderSource(fs,
    'precision mediump float;' +
    'varying vec2 v_uv;' +
    'uniform sampler2D u_tex;' +
    'void main(){gl_FragColor=texture2D(u_tex,v_uv);}'
  )
  gl.value.compileShader(fs)

  const prog = gl.value.createProgram()
  gl.value.attachShader(prog, vs)
  gl.value.attachShader(prog, fs)
  gl.value.linkProgram(prog)
  gl.value.useProgram(prog)

  const buf = gl.value.createBuffer()
  gl.value.bindBuffer(gl.value.ARRAY_BUFFER, buf)
  gl.value.bufferData(gl.value.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.value.STATIC_DRAW)

  const aPos = gl.value.getAttribLocation(prog, 'a_pos')
  gl.value.enableVertexAttribArray(aPos)
  gl.value.vertexAttribPointer(aPos, 2, gl.value.FLOAT, false, 0, 0)

  glFilter.value = props.interpolate ? gl.value.LINEAR : gl.value.NEAREST
  glTex.value = gl.value.createTexture()
  gl.value.bindTexture(gl.value.TEXTURE_2D, glTex.value)
  gl.value.texParameteri(gl.value.TEXTURE_2D, gl.value.TEXTURE_MIN_FILTER, glFilter.value)
  gl.value.texParameteri(gl.value.TEXTURE_2D, gl.value.TEXTURE_MAG_FILTER, glFilter.value)
  gl.value.texParameteri(gl.value.TEXTURE_2D, gl.value.TEXTURE_WRAP_S, gl.value.CLAMP_TO_EDGE)
  gl.value.texParameteri(gl.value.TEXTURE_2D, gl.value.TEXTURE_WRAP_T, gl.value.CLAMP_TO_EDGE)
  gl.value.texImage2D(gl.value.TEXTURE_2D, 0, gl.value.RGBA, W.value, H.value, 0, gl.value.RGBA, gl.value.UNSIGNED_BYTE, null)
  gl.value.viewport(0, 0, W.value, H.value)
  gl.value.clearColor(0, 0, 0, 1)
  gl.value.clear(gl.value.COLOR_BUFFER_BIT)
  gl.value.drawArrays(gl.value.TRIANGLE_STRIP, 0, 4)
}

function glPutTile(x, y, w, h, rgba) {
  if (!gl.value) return
  gl.value.texSubImage2D(gl.value.TEXTURE_2D, 0, x, y, w, h, gl.value.RGBA, gl.value.UNSIGNED_BYTE, rgba)
  glNeedsRedraw.value = true
}

function glFlush() {
  if (glNeedsRedraw.value && gl.value) {
    gl.value.drawArrays(gl.value.TRIANGLE_STRIP, 0, 4)
    glNeedsRedraw.value = false
  }
}

function glResize(w, h) {
  if (!gl.value) return
  gl.value.viewport(0, 0, w, h)
  gl.value.texImage2D(gl.value.TEXTURE_2D, 0, gl.value.RGBA, w, h, 0, gl.value.RGBA, gl.value.UNSIGNED_BYTE, null)
  gl.value.clearColor(0, 0, 0, 1)
  gl.value.clear(gl.value.COLOR_BUFFER_BIT)
}

// 音频相关函数
function ensureAudioCtx(rate) {
  if (audioCtx.value) {
    if (audioCtx.value.sampleRate === rate) return
    if (audioNode.value) {
      audioNode.value.disconnect()
      audioNode.value = null
    }
    audioCtx.value.close()
    audioCtx.value = null
    audioUseWorklet.value = false
    audioRingL.value = null
    audioRingR.value = null
  }
  audioPreBufThreshold.value = Math.max(128, Math.round((rate * audioBufMs.value) / 1000))
  audioCtx.value = new (window.AudioContext || window.webkitAudioContext)({ sampleRate: rate })
  loadAudioWorklet()
  audioCtx.value.resume().then(() => { audioResumed.value = true })
}

function loadAudioWorklet() {
  if (!audioCtx.value) return
  if (audioCtx.value.audioWorklet) {
    const code = audioWorkletCode.replace('8192', '' + audioPreBufThreshold.value)
    const blob = new Blob([code], { type: 'application/javascript' })
    const url = URL.createObjectURL(blob)
    audioCtx.value.audioWorklet.addModule(url).then(() => {
      URL.revokeObjectURL(url)
      if (!audioCtx.value) return
      audioNode.value = new AudioWorkletNode(audioCtx.value, 'audio-output-processor', { outputChannelCount: [2] })
      audioNode.value.connect(audioCtx.value.destination)
      audioUseWorklet.value = true
    }).catch(() => { setupAudioScriptProcessor() })
  } else {
    setupAudioScriptProcessor()
  }
}

function setupAudioScriptProcessor() {
  if (!audioCtx.value) return
  audioRingL.value = new Float32Array(audioRingSize)
  audioRingR.value = new Float32Array(audioRingSize)
  audioWritePos.value = 0
  audioReadPos.value = 0
  audioPreBuffered.value = false
  audioUseWorklet.value = false

  audioNode.value = audioCtx.value.createScriptProcessor(4096, 0, 2)
  audioNode.value.onaudioprocess = (e) => {
    const outL = e.outputBuffer.getChannelData(0)
    const outR = e.outputBuffer.getChannelData(1)
    const avail = (audioWritePos.value - audioReadPos.value) & audioRingMask
    if (!audioPreBuffered.value) {
      if (avail < audioPreBufThreshold.value) {
        for (let i = 0; i < outL.length; i++) { outL[i] = 0; outR[i] = 0 }
        return
      }
      audioPreBuffered.value = true
    }
    const count = Math.min(avail, outL.length)
    let rp = audioReadPos.value
    for (let i = 0; i < count; i++) {
      const idx = rp & audioRingMask
      outL[i] = audioRingL.value[idx]
      outR[i] = audioRingR.value[idx]
      rp++
    }
    for (let i = count; i < outL.length; i++) { outL[i] = 0; outR[i] = 0 }
    audioReadPos.value = rp & audioRingMask
    if (count < outL.length) audioPreBuffered.value = false
  }
  audioNode.value.connect(audioCtx.value.destination)
}

function initAudioOnGesture() {
  if (audioCtx.value) {
    if (!audioResumed.value) {
      audioCtx.value.resume().then(() => { audioResumed.value = true })
    }
    return
  }
  let defaultRate = 44100
  try {
    const tmp = new (window.AudioContext || window.webkitAudioContext)()
    defaultRate = tmp.sampleRate
    tmp.close()
  } catch (e) {}
  ensureAudioCtx(defaultRate)
}

// 麦克风相关函数
function stopMic() {
  if (micProcessor.value) {
    micProcessor.value.disconnect()
    if (micProcessor.value.port) micProcessor.value.port.close()
    micProcessor.value = null
  }
  if (micSource.value) {
    micSource.value.disconnect()
    micSource.value = null
  }
  if (micStream.value) {
    micStream.value.getTracks().forEach(t => t.stop())
    micStream.value = null
  }
  if (micAudioCtx.value) {
    micAudioCtx.value.close()
    micAudioCtx.value = null
  }
  micAccumBufs.value = null
  micAccumFrames.value = 0
  micActive.value = false
}

function startMic(channels, rate, bps, bufMs) {
  stopMic()
  navigator.mediaDevices.getUserMedia({
    audio: { echoCancellation: true, noiseSuppression: true }
  }).then((stream) => {
    micStream.value = stream
    micAudioCtx.value = new (window.AudioContext || window.webkitAudioContext)()
    micBrowserRate.value = micAudioCtx.value.sampleRate
    const ms = !bufMs || bufMs <= 0 ? 10 : bufMs
    micAccumTarget.value = Math.round((micBrowserRate.value * ms) / 1000)
    micAccumBufs.value = null
    micAccumFrames.value = 0
    micSource.value = micAudioCtx.value.createMediaStreamSource(stream)

    if (micAudioCtx.value.audioWorklet) {
      const workletCode = 'class MicProcessor extends AudioWorkletProcessor{process(inputs){if(inputs[0]&&inputs[0].length>0){var out=[];for(var i=0;i<inputs[0].length;i++){out.push(new Float32Array(inputs[0][i]));}this.port.postMessage(out);}return true;}}registerProcessor("mic-processor",MicProcessor);'
      const blob = new Blob([workletCode], { type: 'application/javascript' })
      const url = URL.createObjectURL(blob)
      micAudioCtx.value.audioWorklet.addModule(url).then(() => {
        URL.revokeObjectURL(url)
        if (!micAudioCtx.value) return
        const workletNode = new AudioWorkletNode(micAudioCtx.value, 'mic-processor')
        workletNode.port.onmessage = (e) => { accumulateAndSend(e.data) }
        micSource.value.connect(workletNode)
        workletNode.connect(micAudioCtx.value.destination)
        micProcessor.value = workletNode
        micActive.value = true
      }).catch(() => { setupMicScriptProcessor() })
    } else {
      setupMicScriptProcessor()
    }
  }).catch((err) => { console.warn('[mic] getUserMedia failed:', err) })
}

function setupMicScriptProcessor() {
  if (!micAudioCtx.value || !micSource.value) return
  const proc = micAudioCtx.value.createScriptProcessor(4096, 2, 2)
  proc.onaudioprocess = (e) => {
    const chans = []
    for (let ch = 0; ch < e.inputBuffer.numberOfChannels; ch++) {
      chans.push(new Float32Array(e.inputBuffer.getChannelData(ch)))
    }
    accumulateAndSend(chans)
  }
  micSource.value.connect(proc)
  proc.connect(micAudioCtx.value.destination)
  micProcessor.value = proc
  micActive.value = true
}

function accumulateAndSend(inputChannels) {
  const srcCh = inputChannels.length
  const frames = inputChannels[0].length
  if (!micAccumBufs.value) {
    micAccumBufs.value = []
    for (let c = 0; c < srcCh; c++) {
      micAccumBufs.value.push(new Float32Array(micAccumTarget.value + 256))
    }
    micAccumFrames.value = 0
  }
  if (micAccumFrames.value + frames > micAccumBufs.value[0].length) {
    for (let c = 0; c < srcCh; c++) {
      const nb = new Float32Array(micAccumFrames.value + frames + 256)
      nb.set(micAccumBufs.value[c])
      micAccumBufs.value[c] = nb
    }
  }
  for (let c = 0; c < srcCh; c++) {
    micAccumBufs.value[c].set(inputChannels[c], micAccumFrames.value)
  }
  micAccumFrames.value += frames
  while (micAccumFrames.value >= micAccumTarget.value) {
    const sendChans = []
    for (let c = 0; c < srcCh; c++) {
      sendChans.push(micAccumBufs.value[c].subarray(0, micAccumTarget.value))
    }
    sendMicPCM(sendChans)
    const remain = micAccumFrames.value - micAccumTarget.value
    for (let c = 0; c < srcCh; c++) {
      micAccumBufs.value[c].copyWithin(0, micAccumTarget.value, micAccumTarget.value + remain)
    }
    micAccumFrames.value = remain
  }
}

function sendMicPCM(inputChannels) {
  if (!ws.value || ws.value.readyState !== WebSocket.OPEN) return
  const srcCh = inputChannels.length
  const frames = inputChannels[0].length
  const headerSize = 1 + 4 + 2
  const pcmBytes = frames * srcCh * 2
  const buf = new Uint8Array(headerSize + pcmBytes)
  const dv = new DataView(buf.buffer)
  buf[0] = 0x07
  dv.setUint32(1, micBrowserRate.value, true)
  dv.setUint16(5, srcCh, true)
  let off = headerSize
  for (let i = 0; i < frames; i++) {
    for (let ch = 0; ch < srcCh; ch++) {
      let s = inputChannels[ch][i]
      if (s > 1.0) s = 1.0
      if (s < -1.0) s = -1.0
      dv.setInt16(off, (s * 32767) | 0, true)
      off += 2
    }
  }
  ws.value.send(buf)
}

// 摄像头相关函数
function stopCamera() {
  camRunning.value = false
  if (camReader.value) {
    camReader.value.cancel().catch(() => {})
    camReader.value = null
  }
  if (camEncoder.value && camEncoder.value.state !== 'closed') {
    camEncoder.value.close()
    camEncoder.value = null
  }
  if (camStream.value) {
    camStream.value.getTracks().forEach(t => t.stop())
    camStream.value = null
  }
}

function startCamera(width, height, fps) {
  stopCamera()
  if (typeof VideoEncoder === 'undefined') {
    console.warn('[cam] VideoEncoder not supported')
    return
  }
  camRunning.value = true
  let camFrameCount = 0
  const camKeyInterval = fps * 2
  navigator.mediaDevices.getUserMedia({
    video: { width: { ideal: width }, height: { ideal: height }, frameRate: { ideal: fps } }
  }).then((stream) => {
    if (!camRunning.value) {
      stream.getTracks().forEach(t => t.stop())
      return
    }
    camStream.value = stream
    const track = stream.getVideoTracks()[0]
    const settings = track.getSettings()
    const encW = settings.width || width
    const encH = settings.height || height

    camEncoder.value = new VideoEncoder({
      output: (chunk) => {
        const buf = new Uint8Array(1 + chunk.byteLength)
        buf[0] = 0x0c
        chunk.copyTo(buf.subarray(1))
        if (ws.value && ws.value.readyState === 1) ws.value.send(buf.buffer)
      },
      error: (e) => { console.error('[cam] encoder error:', e) }
    })
    camEncoder.value.configure({
      codec: 'avc1.420028', width: encW, height: encH, framerate: fps,
      bitrate: encH >= 720 ? 1500000 : 700000, latencyMode: 'realtime',
      avc: { format: 'annexb' }
    })

    if (typeof MediaStreamTrackProcessor !== 'undefined') {
      const processor = new MediaStreamTrackProcessor({ track })
      camReader.value = processor.readable.getReader()
      const readFrame = () => {
        if (!camRunning.value) return
        camReader.value.read().then((result) => {
          if (result.done || !camRunning.value) {
            if (result.value) result.value.close()
            return
          }
          const frame = result.value
          if (camEncoder.value && camEncoder.value.state === 'configured') {
            const isKey = camFrameCount % camKeyInterval === 0
            camEncoder.value.encode(frame, { keyFrame: isKey })
            camFrameCount++
          }
          frame.close()
          readFrame()
        }).catch((e) => { console.error('[cam] readFrame error:', e) })
      }
      readFrame()
    } else {
      const vid = document.createElement('video')
      vid.srcObject = stream
      vid.muted = true
      vid.play()
      const camCanvas = document.createElement('canvas')
      camCanvas.width = encW
      camCanvas.height = encH
      const camCtx = camCanvas.getContext('2d')
      const grabFrame = () => {
        if (!camRunning.value || !camEncoder.value || camEncoder.value.state !== 'configured') return
        camCtx.drawImage(vid, 0, 0, encW, encH)
        const frame = new VideoFrame(camCanvas, { timestamp: camFrameCount * (1000000 / fps) })
        const isKey = camFrameCount % camKeyInterval === 0
        camEncoder.value.encode(frame, { keyFrame: isKey })
        camFrameCount++
        frame.close()
      }
      if (vid.requestVideoFrameCallback) {
        const rvfc = () => { if (!camRunning.value) return; grabFrame(); vid.requestVideoFrameCallback(rvfc) }
        vid.requestVideoFrameCallback(rvfc)
      } else {
        const camInterval = setInterval(() => {
          if (!camRunning.value) { clearInterval(camInterval); return }
          grabFrame()
        }, Math.round(1000 / fps))
      }
    }
  }).catch((err) => { console.warn('[cam] getUserMedia failed:', err) })
}

// H.264 解码器相关函数
function isH264Keyframe(nalData) {
  let i = 0
  const len = nalData.length
  while (i < len - 3) {
    if (nalData[i] === 0 && nalData[i + 1] === 0) {
      let nalTypeOff
      if (nalData[i + 2] === 1) {
        nalTypeOff = i + 3
      } else if (nalData[i + 2] === 0 && i + 3 < len && nalData[i + 3] === 1) {
        nalTypeOff = i + 4
      } else {
        i++
        continue
      }
      if (nalTypeOff < len) {
        const nalType = nalData[nalTypeOff] & 0x1f
        if (nalType === 5 || nalType === 7) return true
      }
      i = nalTypeOff
    } else {
      i++
    }
  }
  return false
}

function resetH264Decoder() {
  if (h264Decoder.value && h264Decoder.value.state !== 'closed') h264Decoder.value.close()
  if (h264LumaDecoder.value && h264LumaDecoder.value.state !== 'closed') h264LumaDecoder.value.close()
  h264Decoder.value = null
  h264LumaDecoder.value = null
  h264Timestamp.value = 0
  h264RectMap.value = {}
  h264GotKeyframe.value = false
  h264LumaGotKeyframe.value = false
}

function h264OutputCallback(frame) {
  const ts = frame.timestamp
  const regions = h264RectMap.value[ts]
  delete h264RectMap.value[ts]
  if (!regions) { frame.close(); return }

  const fw = frame.displayWidth
  const fh = frame.displayHeight
  if (gl.value) {
    if (!h264Canvas.value) {
      h264Canvas.value = document.createElement('canvas')
      h264Ctx.value = h264Canvas.value.getContext('2d')
    }
    if (h264Canvas.value.width !== fw || h264Canvas.value.height !== fh) {
      h264Canvas.value.width = fw
      h264Canvas.value.height = fh
    }
    h264Ctx.value.drawImage(frame, 0, 0)
    for (let i = 0; i < regions.length; i++) {
      const r = regions[i]
      let rw = r.right - r.left
      let rh = r.bottom - r.top
      if (r.left + rw > fw) rw = fw - r.left
      if (r.top + rh > fh) rh = fh - r.top
      if (rw > 0 && rh > 0) {
        const px = h264Ctx.value.getImageData(r.left, r.top, rw, rh)
        glPutTile(r.left, r.top, rw, rh, new Uint8Array(px.data.buffer))
      }
    }
    glFlush()
  } else {
    for (let i = 0; i < regions.length; i++) {
      const r = regions[i]
      let rw = r.right - r.left
      let rh = r.bottom - r.top
      if (r.left + rw > fw) rw = fw - r.left
      if (r.top + rh > fh) rh = fh - r.top
      if (rw > 0 && rh > 0) {
        ctx.value.drawImage(frame, r.left, r.top, rw, rh, r.left, r.top, rw, rh)
      }
    }
  }
  frame.close()
}

function createH264Decoder() {
  const dec = new VideoDecoder({
    output: h264OutputCallback,
    error: (e) => {
      console.error('H.264 decoder error:', e)
      if (h264Decoder.value === dec) {
        h264Decoder.value = null
        h264GotKeyframe.value = false
      }
    }
  })
  dec.configure({ codec: 'avc1.4d0028', optimizeForLatency: true })
  return dec
}

// 键盘相关函数
function sendKey(code, pressed) {
  const sc = scancodeMap[code]
  if (sc === undefined) return
  let flags = 0
  if (!pressed) flags |= 0x8000
  if (extendedKeys.has(code)) flags |= 0x0100
  const buf = new Uint8Array(5)
  buf[0] = 0x01
  buf[1] = sc & 0xff
  buf[2] = (sc >> 8) & 0xff
  buf[3] = flags & 0xff
  buf[4] = (flags >> 8) & 0xff
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

function sendUnicode(codepoint, pressed) {
  let flags = 0
  if (!pressed) flags |= 0x8000
  const buf = new Uint8Array(5)
  buf[0] = 0x06
  buf[1] = codepoint & 0xff
  buf[2] = (codepoint >> 8) & 0xff
  buf[3] = flags & 0xff
  buf[4] = (flags >> 8) & 0xff
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

function sendMouse(x, y) {
  const buf = new Uint8Array(7)
  buf[0] = 0x02
  buf[1] = x & 0xff
  buf[2] = (x >> 8) & 0xff
  buf[3] = y & 0xff
  buf[4] = (y >> 8) & 0xff
  const flags = 0x0800 | mouseButtons.value
  buf[5] = flags & 0xff
  buf[6] = (flags >> 8) & 0xff
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

// 事件处理器
const handleKeyDown = (e) => {
  if (e.code === 'KeyV' && (e.ctrlKey || e.metaKey)) return
  if (isMac.value && e.metaKey && e.code === 'KeyC') {
    sendKey('ControlLeft', true)
    sendKey('KeyC', true)
    sendKey('KeyC', false)
    sendKey('ControlLeft', false)
    return
  }
  if (isMac.value && (e.code === 'MetaLeft' || e.code === 'MetaRight')) {
    e.preventDefault()
    return
  }
  if (e.code === 'F11') return
  e.preventDefault()
  if (props.kb === 'unicode' && !controlKeyCodes.has(e.code) && !e.ctrlKey && !e.altKey && !e.metaKey) {
    if (e.key.length === 1) {
      const cp = e.key.charCodeAt(0)
      sendUnicode(cp, true)
      sendUnicode(cp, false)
    }
    return
  }
  sendKey(e.code, true)
}

const handleKeyUp = (e) => {
  if (e.code === 'F11') return
  if (isMac.value && (e.code === 'MetaLeft' || e.code === 'MetaRight')) return
  e.preventDefault()
  if (props.kb === 'unicode' && !controlKeyCodes.has(e.code) && !e.ctrlKey && !e.altKey && !e.metaKey) return
  sendKey(e.code, false)
}

const handlePaste = async (e) => {
  e.preventDefault()
  const text = (e.clipboardData || window.clipboardData).getData('text/plain')
  
  if (!text || !ws.value || ws.value.readyState !== WebSocket.OPEN) return
  
  console.log('📋 粘贴文本长度:', text.length)
  
  // 1. 发送剪贴板内容
  const encoded = new TextEncoder().encode(text)
  const msg = new Uint8Array(1 + encoded.length)
  msg[0] = 0x05
  msg.set(encoded, 1)
  ws.value.send(msg)
  
  // 2. 等待剪贴板同步
  await new Promise(resolve => setTimeout(resolve, 100))
  
  // 3. 发送快捷键
  if (isMac.value) {
    // Mac: Command+V
    sendKey('MetaLeft', true)
    await new Promise(r => setTimeout(r, 20))
    sendKey('KeyV', true)
    await new Promise(r => setTimeout(r, 20))
    sendKey('KeyV', false)
    await new Promise(r => setTimeout(r, 20))
    sendKey('MetaLeft', false)
  } else {
    // Windows/Linux: Ctrl+V
    sendKey('ControlLeft', true)
    await new Promise(r => setTimeout(r, 20))
    sendKey('KeyV', true)
    await new Promise(r => setTimeout(r, 20))
    sendKey('KeyV', false)
    await new Promise(r => setTimeout(r, 20))
    sendKey('ControlLeft', false)
  }
  
  console.log('✅ 粘贴完成')
}
const handleCopy = (e) => {
  if (remoteClipText.value) {
    e.preventDefault()
    e.clipboardData.setData('text/plain', remoteClipText.value)
  }
}

const handleMouseMove = (e) => {
  const rect = canvasRef.value.getBoundingClientRect()
  const x = Math.round((e.clientX - rect.left) * (W.value / rect.width))
  const y = Math.round((e.clientY - rect.top) * (H.value / rect.height))
  sendMouse(x, y)
}

const handleMouseDown = (e) => {
  e.preventDefault()
  const rect = canvasRef.value.getBoundingClientRect()
  const x = Math.round((e.clientX - rect.left) * (W.value / rect.width))
  const y = Math.round((e.clientY - rect.top) * (H.value / rect.height))
  let btn = 0
  if (e.button === 0) btn = 0x1000
  else if (e.button === 2) btn = 0x2000
  else if (e.button === 1) btn = 0x4000
  mouseButtons.value = btn
  const flags = 0x8000 | btn
  const buf = new Uint8Array(7)
  buf[0] = 0x02
  buf[1] = x & 0xff
  buf[2] = (x >> 8) & 0xff
  buf[3] = y & 0xff
  buf[4] = (y >> 8) & 0xff
  buf[5] = flags & 0xff
  buf[6] = (flags >> 8) & 0xff
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

const handleMouseUp = (e) => {
  e.preventDefault()
  const rect = canvasRef.value.getBoundingClientRect()
  const x = Math.round((e.clientX - rect.left) * (W.value / rect.width))
  const y = Math.round((e.clientY - rect.top) * (H.value / rect.height))
  let btn = 0
  if (e.button === 0) btn = 0x1000
  else if (e.button === 2) btn = 0x2000
  else if (e.button === 1) btn = 0x4000
  mouseButtons.value = 0
  const flags = btn
  const buf = new Uint8Array(7)
  buf[0] = 0x02
  buf[1] = x & 0xff
  buf[2] = (x >> 8) & 0xff
  buf[3] = y & 0xff
  buf[4] = (y >> 8) & 0xff
  buf[5] = flags & 0xff
  buf[6] = (flags >> 8) & 0xff
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

const handleWheel = (e) => {
  e.preventDefault()
  const rect = canvasRef.value.getBoundingClientRect()
  const x = Math.round((e.clientX - rect.left) * (W.value / rect.width))
  const y = Math.round((e.clientY - rect.top) * (H.value / rect.height))
  let delta = Math.round(-e.deltaY / 100) || (e.deltaY < 0 ? 1 : -1)
  let horiz = 0
  if (e.deltaX !== 0 && e.deltaY === 0) {
    delta = Math.round(-e.deltaX / 100) || (e.deltaX < 0 ? 1 : -1)
    horiz = 1
  }
  const buf = new Uint8Array(8)
  buf[0] = 0x03
  buf[1] = x & 0xff
  buf[2] = (x >> 8) & 0xff
  buf[3] = y & 0xff
  buf[4] = (y >> 8) & 0xff
  const d16 = delta & 0xffff
  buf[5] = d16 & 0xff
  buf[6] = (d16 >> 8) & 0xff
  buf[7] = horiz
  if (ws.value && ws.value.readyState === WebSocket.OPEN) ws.value.send(buf)
}

const handleResize = () => {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    dpr.value = props.nodpr ? 1 : window.devicePixelRatio || 1
    const newW = Math.round(window.innerWidth * dpr.value)
    const newH = Math.round(window.innerHeight * dpr.value)
    if (newW === W.value && newH === H.value) return
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) return
    const buf = new Uint8Array(5)
    buf[0] = 0x04
    buf[1] = newW & 0xff
    buf[2] = (newW >> 8) & 0xff
    buf[3] = newH & 0xff
    buf[4] = (newH >> 8) & 0xff
    ws.value.send(buf)
    W.value = newW
    H.value = newH
    const canvas = canvasRef.value
    canvas.width = W.value
    canvas.height = H.value
    canvas.style.width = W.value / dpr.value + 'px'
    canvas.style.height = H.value / dpr.value + 'px'
    if (gl.value) glResize(W.value, H.value)
  }, 200)
}

// 初始化
function initDimensions() {
  dpr.value = props.nodpr ? 1 : window.devicePixelRatio || 1
  W.value = props.w || Math.round(window.innerWidth * dpr.value)
  H.value = props.h || Math.round(window.innerHeight * dpr.value)
}

function showOverlay(text, loading = false) {
  overlayText.value = text
  overlayLoading.value = loading
  overlayVisible.value = true
}

function hideOverlay() {
  overlayVisible.value = false
}

function hasSuccessCode(responseData) {
  if (!responseData || typeof responseData !== 'object') {
    return false
  }

  if (!Object.prototype.hasOwnProperty.call(responseData, 'code')) {
    return true
  }

  return Number(responseData.code) === 200
}

async function ensureSession() {
  if (props.wsUrl) {
    return null
  }

  if (sessionIdFromQuery.value) {
    const directSession = {
      sessionId: String(sessionIdFromQuery.value),
      rdpid: String(rdpID.value || '')
    }
    activeSession.value = directSession
    return directSession
  }

  if (!rdpID.value) {
    throw new Error('Missing RDP target')
  }

  if (activeSession.value?.sessionId) {
    return activeSession.value
  }

  clearCachedRDPSession(rdpID.value)
  showOverlay('Preparing session...', true)
  const { data: res } = await API.openRDPSession(rdpID.value)
  if (!hasSuccessCode(res)) {
    throw new Error(res.message || 'Failed to open RDP session')
  }

  const session = extractRDPSession(res)
  if (!session?.sessionId) {
    throw new Error('RDP sessionId missing')
  }

  activeSession.value = session
  return activeSession.value
}

function buildDefaultWsUrl(session) {
  const token = storage.getToken()
  if (!token) return ''

  return buildRdpWsUrl({
    sessionId: session?.sessionId,
    token,
    w: W.value,
    h: H.value,
    monitor: props.monitor || '0',
    scale: props.scale,
    devscale: props.devscale,
    avc: typeof VideoDecoder !== 'undefined'
  })
}

async function connectWebSocket() {
  try {
    const session = await ensureSession()
    const wsUrl = props.wsUrl || buildDefaultWsUrl(session)
    if (!wsUrl) {
      showOverlay('Authentication required')
      emit('error')
      return
    }

    showOverlay('Connecting...', true)

    console.log('Connecting RDP WebSocket', {
      rdpid: rdpID.value,
      sessionId: session?.sessionId || sessionIdFromQuery.value || '',
      width: W.value,
      height: H.value
    })
    ws.value = new WebSocket(wsUrl)
    ws.value.binaryType = 'arraybuffer'

    ws.value.onopen = () => {
      hideOverlay()
      emit('connected')
      const rendererMsg = gl.value
        ? 'canvas: using WebGL (' + gl.value.getParameter(gl.value.VERSION) + ') filter=' + (glFilter.value === (gl.value?.NEAREST || 0) ? 'NEAREST' : 'LINEAR')
        : 'canvas: using Canvas 2D fallback'
      const enc = new TextEncoder()
      const txt = enc.encode(rendererMsg)
      const logBuf = new Uint8Array(1 + txt.length)
      logBuf[0] = 0x0d
      logBuf.set(txt, 1)
      ws.value.send(logBuf)
    }

    ws.value.onclose = () => {
      stopMic()
      showOverlay('Disconnected')
      emit('disconnected')
    }

    ws.value.onerror = () => {
      showOverlay('Connection error')
      emit('error')
    }

    ws.value.onmessage = (ev) => {
      const buf = new Uint8Array(ev.data)
      if (buf.length < 2) return

      const msgType = buf[0]
      if (msgType === 0x01) {
      let off = 0
      while (off + 9 <= buf.length && buf[off] === 0x01) {
        const x = buf[off + 1] | (buf[off + 2] << 8)
        const y = buf[off + 3] | (buf[off + 4] << 8)
        const w = buf[off + 5] | (buf[off + 6] << 8)
        const h = buf[off + 7] | (buf[off + 8] << 8)
        const size = w * h * 4
        if (off + 9 + size > buf.length) break
        const rgba = buf.subarray(off + 9, off + 9 + size)
        if (gl.value) {
          glPutTile(x, y, w, h, rgba)
        } else {
          const img = new ImageData(new Uint8ClampedArray(rgba.buffer, rgba.byteOffset, size), w, h)
          ctx.value.putImageData(img, x, y)
        }
        off += 9 + size
      }
      if (gl.value) glFlush()
    } else if (msgType === 0x03) {
      showOverlay('Disconnected')
      ws.value.close()
    } else if (msgType === 0x04) {
      if (buf.length < 5) return
      const newW = buf[1] | (buf[2] << 8)
      const newH = buf[3] | (buf[4] << 8)
      W.value = newW
      H.value = newH
      const canvas = canvasRef.value
      canvas.width = W.value
      canvas.height = H.value
      canvas.style.width = W.value / dpr.value + 'px'
      canvas.style.height = H.value / dpr.value + 'px'
      if (gl.value) glResize(W.value, H.value)
      else { ctx.value.fillStyle = '#000'; ctx.value.fillRect(0, 0, W.value, H.value) }
      resetH264Decoder()
    } else if (msgType === 0x05) {
      remoteClipText.value = new TextDecoder().decode(buf.subarray(1))
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(remoteClipText.value).catch(() => {})
      }
    } else if (msgType === 0x02) {
      const subtype = buf[1]
      const canvas = canvasRef.value
      if (subtype === 0x00) {
        canvas.style.cursor = 'none'
      } else if (subtype === 0x01) {
        canvas.style.cursor = 'default'
      } else if (subtype === 0x02) {
        if (buf.length < 12) return
        const idx = buf[2] | (buf[3] << 8)
        const hotX = buf[4] | (buf[5] << 8)
        const hotY = buf[6] | (buf[7] << 8)
        const cw = buf[8] | (buf[9] << 8)
        const ch = buf[10] | (buf[11] << 8)
        const cdata = buf.subarray(12)
        if (cdata.length < cw * ch * 4) return
        const cssHotX = Math.round(hotX / dpr.value)
        const cssHotY = Math.round(hotY / dpr.value)
        if (!cursorCanvas.value) {
          cursorCanvas.value = document.createElement('canvas')
          cursorCtx.value = cursorCanvas.value.getContext('2d')
        }
        cursorCanvas.value.width = cw
        cursorCanvas.value.height = ch
        const cimg = new ImageData(new Uint8ClampedArray(cdata.buffer, cdata.byteOffset, cw * ch * 4), cw, ch)
        cursorCtx.value.putImageData(cimg, 0, 0)
        const url = cursorCanvas.value.toDataURL('image/png')
        cursorCache.value[idx] = { url, hotX: cssHotX, hotY: cssHotY }
        canvas.style.cursor = 'url(' + url + ') ' + cssHotX + ' ' + cssHotY + ', auto'
      } else if (subtype === 0x03) {
        if (buf.length < 4) return
        const cidx = buf[2] | (buf[3] << 8)
        const cached = cursorCache.value[cidx]
        if (cached) {
          canvas.style.cursor = 'url(' + cached.url + ') ' + cached.hotX + ' ' + cached.hotY + ', auto'
        }
      }
    } else if (msgType === 0x06) {
      if (buf.length < 10) return
      const channels = buf[1] | (buf[2] << 8)
      const dv = new DataView(ev.data)
      const rate = dv.getUint32(3, true)
      const bps = buf[7] | (buf[8] << 8)
      const pcm = buf.subarray(9)
      ensureAudioCtx(rate)

      let left, right
      if (bps === 16) {
        const frames = (pcm.length / (2 * channels)) | 0
        let i16
        if ((pcm.byteOffset & 1) === 0) {
          i16 = new Int16Array(pcm.buffer, pcm.byteOffset, frames * channels)
        } else {
          const aligned = new ArrayBuffer(frames * channels * 2)
          new Uint8Array(aligned).set(pcm.subarray(0, frames * channels * 2))
          i16 = new Int16Array(aligned)
        }
        left = new Float32Array(frames)
        right = new Float32Array(frames)
        if (channels >= 2) {
          for (let i = 0; i < frames; i++) {
            left[i] = i16[i * channels] / 32768.0
            right[i] = i16[i * channels + 1] / 32768.0
          }
        } else {
          for (let i = 0; i < frames; i++) {
            const s = i16[i] / 32768.0
            left[i] = s
            right[i] = s
          }
        }
      } else if (bps === 8) {
        const frames = (pcm.length / channels) | 0
        left = new Float32Array(frames)
        right = new Float32Array(frames)
        if (channels >= 2) {
          for (let i = 0; i < frames; i++) {
            left[i] = (pcm[i * channels] - 128) / 128.0
            right[i] = (pcm[i * channels + 1] - 128) / 128.0
          }
        } else {
          for (let i = 0; i < frames; i++) {
            const s = (pcm[i] - 128) / 128.0
            left[i] = s
            right[i] = s
          }
        }
      }
      if (!left) return

      if (audioUseWorklet.value && audioNode.value) {
        audioNode.value.port.postMessage({ left, right }, [left.buffer, right.buffer])
      } else if (audioRingL.value) {
        let wp = audioWritePos.value
        for (let i = 0; i < left.length; i++) {
          const idx = wp & audioRingMask
          audioRingL.value[idx] = left[i]
          audioRingR.value[idx] = right[i]
          wp++
        }
        audioWritePos.value = wp & audioRingMask
        const avail = (audioWritePos.value - audioReadPos.value) & audioRingMask
        if (avail > audioRingSize - 8192) {
          audioReadPos.value = (audioWritePos.value - 22050) & audioRingMask
        }
      }
    } else if (msgType === 0x07) {
      if (buf.length < 9) return
      const micCh = buf[1] | (buf[2] << 8)
      const micDv = new DataView(ev.data)
      const micRate = micDv.getUint32(3, true)
      const micBps = buf[7] | (buf[8] << 8)
      const bufMs = buf.length >= 11 ? buf[9] | (buf[10] << 8) : 0
      if (micCh === 0 && micRate === 0 && micBps === 0) {
        stopMic()
      } else {
        startMic(micCh, micRate, micBps, bufMs)
      }
    } else if (msgType === 0x0b) {
      if (buf.length < 2) return
      const camSubtype = buf[1]
      if (camSubtype === 0x01 && buf.length >= 8) {
        const camW = buf[2] | (buf[3] << 8)
        const camH = buf[4] | (buf[5] << 8)
        const camFps = buf[6] | (buf[7] << 8)
        startCamera(camW, camH, camFps)
      } else if (camSubtype === 0x02) {
        stopCamera()
      }
    } else if (msgType === 0x0a) {
      if (buf.length < 14) return
      const mode = buf[3]
      if (mode === 2) return
      const numRegions = buf[12] | (buf[13] << 8)
      const nalOff = 14 + numRegions * 10
      if (nalOff > buf.length) return
      const nalData = buf.subarray(nalOff)
      if (nalData.length === 0) return

      const isKey = isH264Keyframe(nalData)
      let dec, gotKey
      if (mode === 1) {
        if (!h264LumaDecoder.value) h264LumaDecoder.value = createH264Decoder()
        dec = h264LumaDecoder.value
        gotKey = h264LumaGotKeyframe.value
        if (!isKey && !gotKey) return
        if (isKey) h264LumaGotKeyframe.value = true
      } else {
        if (!h264Decoder.value) h264Decoder.value = createH264Decoder()
        dec = h264Decoder.value
        gotKey = h264GotKeyframe.value
        if (!isKey && !gotKey) return
        if (isKey) h264GotKeyframe.value = true
      }
      if (dec.state === 'closed') return

      const regions = []
      let roff = 14
      for (let ri = 0; ri < numRegions; ri++) {
        regions.push({
          left: buf[roff] | (buf[roff + 1] << 8),
          top: buf[roff + 2] | (buf[roff + 3] << 8),
          right: buf[roff + 4] | (buf[roff + 5] << 8),
          bottom: buf[roff + 6] | (buf[roff + 7] << 8)
        })
        roff += 10
      }

      const ts = h264Timestamp.value
      h264Timestamp.value += 33333
      h264RectMap.value[ts] = regions

      try {
        const chunk = new EncodedVideoChunk({
          type: isKey ? 'key' : 'delta',
          timestamp: ts,
          data: nalData
        })
        dec.decode(chunk)
      } catch (e) {
        console.error('H.264 decode error (mode=' + mode + '):', e)
        delete h264RectMap.value[ts]
      }
      }
    }
  } catch (error) {
    console.error('RDP session bootstrap failed:', error)
    showOverlay(error.message || 'Connection error')
    overlayLoading.value = false
    emit('error')
  }
}

// 生命周期
onMounted(() => {
  // 设置标题
  if (props.monitor !== null) {
    let monTitle = 'Display ' + props.monitor
    if (props.primary === '1') monTitle += ' (primary)'
    document.title = monTitle
  }

  initDimensions()
  initRenderer()
  
  // 初始化光标画布
  cursorCanvas.value = document.createElement('canvas')
  cursorCtx.value = cursorCanvas.value.getContext('2d')
  
  // 添加事件监听
  document.addEventListener('keydown', handleKeyDown)
  document.addEventListener('keyup', handleKeyUp)
  document.addEventListener('paste', handlePaste)
  document.addEventListener('click', initAudioOnGesture)
  document.addEventListener('keydown', initAudioOnGesture)
  window.addEventListener('resize', handleResize)
  
  if (isMac.value) {
    document.addEventListener('copy', handleCopy)
  }
  
  // 连接 WebSocket
  connectWebSocket()
})

onUnmounted(() => {
  // 清理
  stopMic()
  stopCamera()
  
  if (ws.value) {
    ws.value.close()
    ws.value = null
  }
  
  if (audioCtx.value) {
    audioCtx.value.close()
    audioCtx.value = null
  }
  
  // 移除事件监听
  document.removeEventListener('keydown', handleKeyDown)
  document.removeEventListener('keyup', handleKeyUp)
  document.removeEventListener('paste', handlePaste)
  document.removeEventListener('click', initAudioOnGesture)
  document.removeEventListener('keydown', initAudioOnGesture)
  window.removeEventListener('resize', handleResize)
  
  if (isMac.value) {
    document.removeEventListener('copy', handleCopy)
  }
  
  if (resizeTimer) {
    clearTimeout(resizeTimer)
    resizeTimer = null
  }
})
</script>

<style scoped>
.rdp-client {
  width: 100%;
  height: 100%;
  position: relative;
  background: #1a1a2e;
  overflow: auto;
   display: flex;              /* 新增 */
  justify-content: center;    /* 新增：水平居中 */
  align-items: center;        /* 新增：垂直居中 */
}

canvas {
  cursor: default;
  display: block;
}

.overlay {
  position: fixed;
  inset: 0;
  background: #000;
  color: #cc7000;
  font: bold 48px sans-serif;
  z-index: 20;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.mic-indicator {
  position: fixed;
  top: 8px;
  right: 8px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #e00;
  z-index: 30;
  box-shadow: 0 0 6px #e00;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.spinner {
  width: 48px;
  height: 48px;
  border: 4px solid #331c00;
  border-top-color: #cc7000;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-bottom: 24px;
}

.overlay-text {
  font-size: 48px;
}
</style>
