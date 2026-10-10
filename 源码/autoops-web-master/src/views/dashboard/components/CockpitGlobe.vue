<script setup>
/* global defineProps */
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as THREE from 'three'

const props = defineProps({
  markers: {
    type: Array,
    default: () => []
  }
})

const stageRef = ref(null)
const webglError = ref(false)
const isDragging = ref(false)

let scene
let camera
let renderer
let globeGroup
let markerGroup
let orbitGroup
let cloudLayer
let cloudTexture
let starField
let starMaterial
let shootingStars = []
let resizeObserver
let animationFrame
let texture
let lightTexture
let reducedMotion = false
let lastFrameTime = 0
let pointerStart = { x: 0, y: 0 }
let rotationStart = { x: 0, y: 0 }

const textureUrl = `${process.env.BASE_URL}tv-cockpit/assets/earth_all.109568475b79a63e5fd7.jpg`
const lightTextureUrl = `${process.env.BASE_URL}tv-cockpit/assets/Earth_light.fc1a0132dbcb143042c6.png`

const seededRandom = seed => {
  const value = Math.sin(seed * 12.9898 + 78.233) * 43758.5453
  return value - Math.floor(value)
}

const latLonToVector = (lat, lon, radius = 2.04) => {
  const phi = (90 - Number(lat)) * Math.PI / 180
  const theta = (Number(lon) + 180) * Math.PI / 180
  return new THREE.Vector3(
    -radius * Math.sin(phi) * Math.cos(theta),
    radius * Math.cos(phi),
    radius * Math.sin(phi) * Math.sin(theta)
  )
}

const disposeObject = object => {
  object?.traverse(child => {
    child.geometry?.dispose?.()
    if (Array.isArray(child.material)) child.material.forEach(material => material.dispose?.())
    else child.material?.dispose?.()
  })
}

const render = () => {
  if (!renderer || !scene || !camera) return
  renderer.render(scene, camera)
}

const createStars = () => {
  const geometry = new THREE.BufferGeometry()
  const positions = []
  const phases = []
  const sizes = []
  const warmth = []
  for (let index = 0; index < 920; index += 1) {
    const seed = seededRandom(index * 3 + 1)
    const seed2 = seededRandom(index * 3 + 2)
    const seed3 = seededRandom(index * 3 + 3)
    positions.push(
      (seed - 0.5) * 28,
      (seed2 - 0.5) * 17,
      -2.6 - seed3 * 9
    )
    phases.push(seededRandom(index * 5 + 11) * Math.PI * 2)
    sizes.push(1.6 + seededRandom(index * 7 + 17) * 3.2)
    warmth.push(seededRandom(index * 11 + 23))
  }
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  geometry.setAttribute('aPhase', new THREE.Float32BufferAttribute(phases, 1))
  geometry.setAttribute('aSize', new THREE.Float32BufferAttribute(sizes, 1))
  geometry.setAttribute('aWarmth', new THREE.Float32BufferAttribute(warmth, 1))
  starMaterial = new THREE.ShaderMaterial({
    uniforms: {
      uTime: { value: 0 },
      uPixelRatio: { value: Math.min(window.devicePixelRatio || 1, 2) }
    },
    vertexShader: `
      attribute float aPhase;
      attribute float aSize;
      attribute float aWarmth;
      uniform float uTime;
      uniform float uPixelRatio;
      varying float vBrightness;
      varying float vWarmth;
      void main() {
        vec4 modelViewPosition = modelViewMatrix * vec4(position, 1.0);
        float pulse = 0.72 + 0.28 * sin(uTime * (0.72 + fract(aPhase) * 1.35) + aPhase);
        vBrightness = pulse;
        vWarmth = aWarmth;
        gl_PointSize = aSize * uPixelRatio * (13.0 / max(7.0, -modelViewPosition.z));
        gl_Position = projectionMatrix * modelViewPosition;
      }
    `,
    fragmentShader: `
      varying float vBrightness;
      varying float vWarmth;
      void main() {
        float distanceToCenter = distance(gl_PointCoord, vec2(0.5));
        float halo = 1.0 - smoothstep(0.08, 0.5, distanceToCenter);
        float core = 1.0 - smoothstep(0.0, 0.16, distanceToCenter);
        vec3 cool = vec3(0.42, 0.77, 1.0);
        vec3 warm = vec3(1.0, 0.91, 0.72);
        vec3 color = mix(cool, warm, smoothstep(0.78, 1.0, vWarmth));
        gl_FragColor = vec4(color + core * 0.35, halo * vBrightness * 0.86);
      }
    `,
    transparent: true,
    blending: THREE.AdditiveBlending,
    depthWrite: false
  })
  starField = new THREE.Points(geometry, starMaterial)
  scene.add(starField)
}

const createShootingStars = () => {
  const configurations = [
    {
      start: new THREE.Vector3(-6.8, 3.2, -3.2),
      end: new THREE.Vector3(4.8, -2.1, -3.2),
      period: 12,
      offset: 0,
      length: 1.7
    },
    {
      start: new THREE.Vector3(6.4, 2.6, -4.4),
      end: new THREE.Vector3(-4.2, -2.7, -4.4),
      period: 17,
      offset: 7.2,
      length: 1.35
    }
  ]

  configurations.forEach(configuration => {
    const direction = configuration.end.clone().sub(configuration.start)
    const material = new THREE.ShaderMaterial({
      uniforms: { uOpacity: { value: 0 } },
      vertexShader: `
        varying vec2 vUv;
        void main() {
          vUv = uv;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: `
        uniform float uOpacity;
        varying vec2 vUv;
        void main() {
          float verticalFade = 1.0 - smoothstep(0.0, 0.48, abs(vUv.y - 0.5));
          float trail = pow(vUv.x, 2.35);
          float head = smoothstep(0.72, 1.0, vUv.x);
          vec3 color = mix(vec3(0.26, 0.69, 1.0), vec3(0.92, 1.0, 1.0), head);
          gl_FragColor = vec4(color, verticalFade * trail * (0.5 + head * 0.5) * uOpacity);
        }
      `,
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false
    })
    const meteor = new THREE.Mesh(new THREE.PlaneGeometry(configuration.length, 0.045), material)
    meteor.position.copy(configuration.start)
    meteor.rotation.z = Math.atan2(direction.y, direction.x)
    meteor.visible = false
    scene.add(meteor)
    shootingStars.push({ ...configuration, material, meteor })
  })
}

const createCloudTexture = () => {
  const canvas = document.createElement('canvas')
  canvas.width = 1024
  canvas.height = 512
  const context = canvas.getContext('2d')
  if (!context) return null

  context.clearRect(0, 0, canvas.width, canvas.height)
  context.filter = 'blur(5px)'
  for (let cluster = 0; cluster < 145; cluster += 1) {
    const baseX = seededRandom(cluster * 13 + 2) * canvas.width
    const latitude = seededRandom(cluster * 17 + 5)
    const baseY = canvas.height * (0.16 + latitude * 0.68)
    const parts = 3 + Math.floor(seededRandom(cluster * 19 + 7) * 5)
    for (let part = 0; part < parts; part += 1) {
      const radiusX = 14 + seededRandom(cluster * 31 + part * 7) * 54
      const radiusY = 3 + seededRandom(cluster * 37 + part * 11) * 12
      const x = baseX + (seededRandom(cluster * 41 + part * 13) - 0.5) * 82
      const y = baseY + (seededRandom(cluster * 43 + part * 17) - 0.5) * 28
      const alpha = 0.05 + seededRandom(cluster * 47 + part * 19) * 0.16
      context.save()
      context.translate(x, y)
      context.scale(radiusX, radiusY)
      const gradient = context.createRadialGradient(0, 0, 0, 0, 0, 1)
      gradient.addColorStop(0, `rgba(238, 248, 255, ${alpha})`)
      gradient.addColorStop(0.58, `rgba(212, 236, 251, ${alpha * 0.7})`)
      gradient.addColorStop(1, 'rgba(190, 224, 245, 0)')
      context.fillStyle = gradient
      context.beginPath()
      context.arc(0, 0, 1, 0, Math.PI * 2)
      context.fill()
      context.restore()
    }
  }
  context.filter = 'none'

  const generatedTexture = new THREE.CanvasTexture(canvas)
  generatedTexture.colorSpace = THREE.SRGBColorSpace
  generatedTexture.wrapS = THREE.RepeatWrapping
  generatedTexture.anisotropy = Math.min(8, renderer?.capabilities.getMaxAnisotropy?.() || 1)
  return generatedTexture
}

const createCloudLayer = () => {
  cloudTexture = createCloudTexture()
  if (!cloudTexture) return
  cloudLayer = new THREE.Mesh(
    new THREE.SphereGeometry(2.035, 96, 96),
    new THREE.MeshPhongMaterial({
      map: cloudTexture,
      color: 0xccecff,
      transparent: true,
      opacity: 0.34,
      alphaTest: 0.015,
      depthWrite: false,
      shininess: 4
    })
  )
  globeGroup.add(cloudLayer)
}

const createAtmosphere = () => {
  const material = new THREE.ShaderMaterial({
    uniforms: {
      glowColor: { value: new THREE.Color(0x2dc8ff) }
    },
    vertexShader: `
      varying vec3 vertexNormal;
      void main() {
        vertexNormal = normalize(normalMatrix * normal);
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform vec3 glowColor;
      varying vec3 vertexNormal;
      void main() {
        float intensity = pow(0.68 - dot(vertexNormal, vec3(0.0, 0.0, 1.0)), 2.7);
        gl_FragColor = vec4(glowColor, intensity * 0.44);
      }
    `,
    blending: THREE.AdditiveBlending,
    side: THREE.BackSide,
    transparent: true,
    depthWrite: false
  })
  const atmosphere = new THREE.Mesh(new THREE.SphereGeometry(2.14, 72, 72), material)
  globeGroup.add(atmosphere)

  const surfaceHaze = new THREE.Mesh(
    new THREE.SphereGeometry(2.055, 72, 72),
    new THREE.MeshBasicMaterial({
      color: 0x3acbff,
      transparent: true,
      opacity: 0.055,
      blending: THREE.AdditiveBlending,
      depthWrite: false
    })
  )
  globeGroup.add(surfaceHaze)
}

const createGeoGrid = () => {
  const gridGroup = new THREE.Group()
  const latitudeMaterial = new THREE.LineBasicMaterial({
    color: 0x4fcfff,
    transparent: true,
    opacity: 0.075,
    depthWrite: false
  })
  const longitudeMaterial = new THREE.LineBasicMaterial({
    color: 0x5ce3ba,
    transparent: true,
    opacity: 0.045,
    depthWrite: false
  })

  for (let lat = -60; lat <= 60; lat += 20) {
    const points = []
    for (let lon = -180; lon <= 180; lon += 3) points.push(latLonToVector(lat, lon, 2.045))
    gridGroup.add(new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(points), latitudeMaterial))
  }

  for (let lon = -150; lon <= 180; lon += 30) {
    const points = []
    for (let lat = -90; lat <= 90; lat += 3) points.push(latLonToVector(lat, lon, 2.045))
    gridGroup.add(new THREE.Line(new THREE.BufferGeometry().setFromPoints(points), longitudeMaterial))
  }

  globeGroup.add(gridGroup)
}

const createOrbitRing = ({ radius, color, opacity, rotation }) => {
  const points = new THREE.EllipseCurve(0, 0, radius, radius, 0, Math.PI * 2, false, 0)
    .getPoints(220)
    .map(point => new THREE.Vector3(point.x, point.y, 0))
  const ring = new THREE.LineLoop(
    new THREE.BufferGeometry().setFromPoints(points),
    new THREE.LineBasicMaterial({ color, transparent: true, opacity, depthWrite: false })
  )
  ring.rotation.set(...rotation)
  return ring
}

const createOrbits = () => {
  orbitGroup = new THREE.Group()
  orbitGroup.add(
    createOrbitRing({ radius: 2.48, color: 0x50d6ff, opacity: 0.28, rotation: [1.08, 0.18, 0.18] }),
    createOrbitRing({ radius: 2.62, color: 0x5ce3ba, opacity: 0.2, rotation: [0.45, 0.55, -0.42] }),
    createOrbitRing({ radius: 2.76, color: 0xffc857, opacity: 0.13, rotation: [1.42, -0.26, 0.54] })
  )

  const satelliteMaterial = new THREE.MeshBasicMaterial({ color: 0xffc857 })
  ;[
    new THREE.Vector3(2.32, 0.72, 0.42),
    new THREE.Vector3(-1.7, 1.62, -0.84),
    new THREE.Vector3(0.55, -2.3, 0.94)
  ].forEach(position => {
    const satellite = new THREE.Mesh(new THREE.SphereGeometry(0.035, 12, 12), satelliteMaterial)
    satellite.position.copy(position)
    orbitGroup.add(satellite)
  })
  scene.add(orbitGroup)
}

const rebuildMarkers = () => {
  if (!globeGroup) return
  if (markerGroup) {
    globeGroup.remove(markerGroup)
    disposeObject(markerGroup)
  }
  markerGroup = new THREE.Group()

  props.markers.forEach(marker => {
    const position = latLonToVector(marker.lat, marker.lon)
    const normal = position.clone().normalize()
    const markerMaterial = new THREE.MeshBasicMaterial({ color: 0xffc857 })
    const point = new THREE.Mesh(new THREE.SphereGeometry(0.052, 16, 16), markerMaterial)
    point.position.copy(position)
    markerGroup.add(point)

    const beamEnd = position.clone().multiplyScalar(1.12)
    const beam = new THREE.Line(
      new THREE.BufferGeometry().setFromPoints([position, beamEnd]),
      new THREE.LineBasicMaterial({ color: 0x5ce3ba, transparent: true, opacity: 0.72 })
    )
    markerGroup.add(beam)

    const beacon = new THREE.Mesh(
      new THREE.SphereGeometry(0.026, 12, 12),
      new THREE.MeshBasicMaterial({ color: 0x5ce3ba })
    )
    beacon.position.copy(beamEnd)
    markerGroup.add(beacon)

    const ring = new THREE.Mesh(
      new THREE.RingGeometry(0.06, 0.085, 28),
      new THREE.MeshBasicMaterial({
        color: 0x52e0a6,
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.85,
        depthWrite: false
      })
    )
    ring.position.copy(position.clone().add(normal.multiplyScalar(0.008)))
    ring.lookAt(position.clone().multiplyScalar(2))
    markerGroup.add(ring)
  })

  globeGroup.add(markerGroup)
  render()
}

const resize = () => {
  if (!stageRef.value || !renderer || !camera) return
  const { clientWidth, clientHeight } = stageRef.value
  if (!clientWidth || !clientHeight) return
  renderer.setSize(clientWidth, clientHeight, false)
  camera.aspect = clientWidth / clientHeight
  camera.updateProjectionMatrix()
  render()
}

const animate = timestamp => {
  if (!renderer) return
  const currentTime = Number(timestamp || 0) * 0.001
  const delta = lastFrameTime ? Math.min(0.05, currentTime - lastFrameTime) : 0
  lastFrameTime = currentTime
  if (!isDragging.value) globeGroup.rotation.y += delta * 0.052
  if (orbitGroup) orbitGroup.rotation.y -= delta * 0.017
  if (cloudLayer) cloudLayer.rotation.y += delta * 0.006
  if (starField) starField.rotation.y += delta * 0.0008
  if (starMaterial) starMaterial.uniforms.uTime.value = currentTime
  shootingStars.forEach(item => {
    const phase = ((currentTime + item.offset) % item.period) / item.period
    const activeRatio = 1.45 / item.period
    if (phase < activeRatio) {
      const progress = phase / activeRatio
      item.meteor.visible = true
      item.meteor.position.lerpVectors(item.start, item.end, progress)
      item.material.uniforms.uOpacity.value = Math.sin(progress * Math.PI) * 0.9
    } else {
      item.meteor.visible = false
    }
  })
  render()
  animationFrame = window.requestAnimationFrame(animate)
}

const initialize = async () => {
  await nextTick()
  if (!stageRef.value) return
  try {
    scene = new THREE.Scene()
    camera = new THREE.PerspectiveCamera(34, 1, 0.1, 100)
    camera.position.set(0, 0.08, 9.0)

    renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'high-performance' })
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.12
    renderer.setClearColor(0x000000, 0)
    stageRef.value.appendChild(renderer.domElement)

    globeGroup = new THREE.Group()
    globeGroup.rotation.set(-0.08, -0.2, 0.02)
    scene.add(globeGroup)

    const textureLoader = new THREE.TextureLoader()
    texture = textureLoader.load(textureUrl, render, undefined, () => {
      texture = null
      render()
    })
    lightTexture = textureLoader.load(lightTextureUrl, render, undefined, () => {
      lightTexture = null
      render()
    })
    if (texture) {
      texture.colorSpace = THREE.SRGBColorSpace
      texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())
    }
    if (lightTexture) {
      lightTexture.colorSpace = THREE.SRGBColorSpace
      lightTexture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())
    }

    const earth = new THREE.Mesh(
      new THREE.SphereGeometry(2, 96, 96),
      new THREE.MeshPhysicalMaterial({
        map: texture,
        emissiveMap: texture,
        color: 0xd4e4f2,
        emissive: 0x183c63,
        emissiveIntensity: 0.34,
        roughness: 0.68,
        metalness: 0.01,
        clearcoat: 0.16,
        clearcoatRoughness: 0.72
      })
    )
    globeGroup.add(earth)

    const earthLights = new THREE.Mesh(
      new THREE.SphereGeometry(2.012, 96, 96),
      new THREE.ShaderMaterial({
        uniforms: { lightMap: { value: lightTexture } },
        vertexShader: `
          varying vec2 vUv;
          void main() {
            vUv = uv;
            gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
          }
        `,
        fragmentShader: `
          uniform sampler2D lightMap;
          varying vec2 vUv;
          void main() {
            vec3 source = texture2D(lightMap, vUv).rgb;
            float cyanSignal = source.b - source.r;
            float brightness = dot(source, vec3(0.16, 0.5, 0.34));
            float mask = smoothstep(0.2, 0.48, cyanSignal) * smoothstep(0.34, 0.82, brightness);
            vec3 glow = mix(vec3(0.1, 0.45, 1.0), vec3(0.36, 0.96, 1.0), smoothstep(0.45, 0.95, source.g));
            gl_FragColor = vec4(glow, mask * 0.68);
          }
        `,
        transparent: true,
        blending: THREE.AdditiveBlending,
        depthWrite: false
      })
    )
    globeGroup.add(earthLights)
    createCloudLayer()
    createGeoGrid()
    createAtmosphere()
    createOrbits()
    createStars()
    createShootingStars()

    scene.add(new THREE.AmbientLight(0x17344c, 0.62))
    scene.add(new THREE.HemisphereLight(0x9ddfff, 0x020812, 0.86))
    const keyLight = new THREE.DirectionalLight(0xf1f8ff, 3.15)
    keyLight.position.set(5.5, 3.6, 5.8)
    scene.add(keyLight)
    const rimLight = new THREE.DirectionalLight(0x28bfff, 0.82)
    rimLight.position.set(-4.5, 0.6, 2.2)
    scene.add(rimLight)

    rebuildMarkers()
    resizeObserver = new ResizeObserver(resize)
    resizeObserver.observe(stageRef.value)
    resize()

    reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (reducedMotion) render()
    else animate()
  } catch (error) {
    console.error('Three.js 地球初始化失败:', error)
    webglError.value = true
  }
}

const onPointerDown = event => {
  if (!globeGroup) return
  isDragging.value = true
  pointerStart = { x: event.clientX, y: event.clientY }
  rotationStart = { x: globeGroup.rotation.x, y: globeGroup.rotation.y }
  stageRef.value?.setPointerCapture?.(event.pointerId)
}

const onPointerMove = event => {
  if (!isDragging.value || !globeGroup) return
  globeGroup.rotation.y = rotationStart.y + (event.clientX - pointerStart.x) * 0.006
  globeGroup.rotation.x = Math.max(-0.8, Math.min(0.8, rotationStart.x + (event.clientY - pointerStart.y) * 0.004))
  render()
}

const onPointerUp = event => {
  isDragging.value = false
  stageRef.value?.releasePointerCapture?.(event.pointerId)
}

const onWheel = event => {
  if (!camera) return
  camera.position.z = Math.max(6.4, Math.min(9.6, camera.position.z + Math.sign(event.deltaY) * 0.25))
  render()
}

watch(() => props.markers, rebuildMarkers, { deep: true })

onMounted(initialize)

onBeforeUnmount(() => {
  window.cancelAnimationFrame(animationFrame)
  resizeObserver?.disconnect()
  disposeObject(scene)
  texture?.dispose?.()
  lightTexture?.dispose?.()
  cloudTexture?.dispose?.()
  renderer?.dispose?.()
  renderer?.domElement?.remove()
  renderer = null
  scene = null
  starMaterial = null
  starField = null
  shootingStars = []
})
</script>

<template>
  <div
    ref="stageRef"
    class="cockpit-globe"
    :class="{ 'is-dragging': isDragging }"
    role="img"
    aria-label="资产机房地理分布地球，可拖动旋转并滚轮缩放"
    tabindex="0"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="onPointerUp"
    @wheel.prevent="onWheel"
  >
    <div
      v-if="webglError"
      class="cockpit-globe__fallback"
      :style="{ backgroundImage: `url('${textureUrl}')` }"
    >
      <span>当前浏览器无法加载 3D 场景</span>
      <small>资产和告警数据仍可正常查看</small>
    </div>
  </div>
</template>

<style scoped lang="scss">
.cockpit-globe {
  position: absolute;
  inset: 0;
  overflow: hidden;
  cursor: grab;
  touch-action: none;
  user-select: none;
}

.cockpit-globe.is-dragging {
  cursor: grabbing;
}

.cockpit-globe:focus-visible {
  outline: 2px solid rgba(84, 210, 255, 0.92);
  outline-offset: -3px;
}

.cockpit-globe :deep(canvas) {
  display: block;
  width: 100%;
  height: 100%;
}

.cockpit-globe__fallback {
  position: absolute;
  inset: 8% 10%;
  display: grid;
  place-content: center;
  gap: 8px;
  color: #d8f4ff;
  text-align: center;
  background-position: center;
  background-repeat: no-repeat;
  background-size: min(58vh, 58vw) auto;
}

.cockpit-globe__fallback small {
  color: #8ca9bc;
}

@media (prefers-reduced-motion: reduce) {
  .cockpit-globe {
    scroll-behavior: auto;
  }
}
</style>
