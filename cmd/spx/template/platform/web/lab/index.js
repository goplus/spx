window.addEventListener('load', function() {
	animateBox();
	initEngine()
});

function animateBox() {
	const box = document.querySelector('.animation-box');
	if (!box) return;
	const style = window.getComputedStyle(box);
	if (style.animationName !== 'none') return;
	let position = 0;
	let direction = 1;
	setInterval(() => {
		if (position >= 250) direction = -1;
		if (position <= 0) direction = 1;

		position += direction * 2;
		box.style.left = position + 'px';
	}, 20);
}

let runnerWindow = {}
document.getElementById('startGameBtn').addEventListener('click', startGame);
document.getElementById('stopGameBtn').addEventListener('click', stopGame);
document.getElementById('pauseGameBtn').addEventListener('click', pauseGame);
document.getElementById('resumeGameBtn').addEventListener('click', resumeGame);
document.getElementById('stepNextFrameBtn').addEventListener('click', stepNextFrame);
document.getElementById('toggleSizeBtn').addEventListener('click', toggleSizeFrame);
document.getElementById('selectCaptureDirBtn').addEventListener('click', selectCaptureDirectory);
document.getElementById('refreshCompareBtn').addEventListener('click', refreshCaptureCompare);

const iframe = document.getElementById('runnerFrame');
let enlarged = false;
const captureState = {
	rootHandle: null,
	project: null,
	session: null,
	refreshTimer: 0,
	previewUrls: [],
}

runnerWindow = iframe.contentWindow;

function updateCaptureStatus(message, isError = false) {
	const status = document.getElementById('captureStatus')
	status.textContent = message
	status.style.color = isError ? '#ff9b9b' : '#fff'
}

function revokeCapturePreviewUrls() {
	captureState.previewUrls.forEach((url) => URL.revokeObjectURL(url))
	captureState.previewUrls = []
}

function setCaptureCompareEmpty(message, summary = message) {
	revokeCapturePreviewUrls()
	document.getElementById('captureCompareSummary').textContent = summary
	document.getElementById('captureCompareGrid').innerHTML = `<div class="capture-compare-empty">${message}</div>`
}

function scheduleCaptureCompareRefresh() {
	if (captureState.refreshTimer) {
		clearTimeout(captureState.refreshTimer)
	}
	captureState.refreshTimer = window.setTimeout(() => {
		captureState.refreshTimer = 0
		refreshCaptureCompare().catch((error) => {
			console.error('Failed to refresh capture compare', error)
			setCaptureCompareEmpty('Failed to read capture files. Check browser console for details.')
		})
	}, 120)
}

function browserSupportsCaptureDirectory() {
	return typeof window.showDirectoryPicker === 'function'
}

function buildCaptureDirectoryPickerOptions() {
	const options = {
		mode: 'readwrite',
		id: 'spx-capture-root',
	}
	if (captureState.rootHandle) {
		options.startIn = captureState.rootHandle
		return options
	}
	options.startIn = 'downloads'
	return options
}

function sanitizeCaptureSegment(value, fallback = 'project') {
	let segment = typeof value === 'string' ? value.trim() : ''
	segment = segment.replace(/[\\/:*?"<>|\u0000-\u001F]+/g, '_')
	return segment || fallback
}

function formatCaptureTimestamp(date = new Date()) {
	const pad2 = (value) => `${value}`.padStart(2, '0')
	return `${date.getFullYear()}${pad2(date.getMonth() + 1)}${pad2(date.getDate())}-${pad2(date.getHours())}${pad2(date.getMinutes())}${pad2(date.getSeconds())}`
}

function decodeZipText(unzipped, path) {
	const entry = unzipped[path]
	if (!entry) {
		return ''
	}
	return new TextDecoder('utf-8').decode(entry)
}

function parseCaptureProject(unzipped) {
	let builderMeta = null
	let projectIndex = null
	try {
		const builderMetaText = decodeZipText(unzipped, 'builder-meta.json')
		if (builderMetaText) {
			builderMeta = JSON.parse(builderMetaText)
		}
	} catch (error) {
		console.warn('Failed to parse builder-meta.json for capture host', error)
	}
	try {
		const projectIndexText = decodeZipText(unzipped, 'assets/index.json')
		if (projectIndexText) {
			projectIndex = JSON.parse(projectIndexText)
		}
	} catch (error) {
		console.warn('Failed to parse assets/index.json for capture host', error)
	}
	const runConfig = projectIndex && typeof projectIndex.run === 'object' ? projectIndex.run : {}
	const fixedTimestep = Number(runConfig.fixedTimestep)
	return {
		projectKey: sanitizeCaptureSegment(
			(builderMeta && builderMeta.displayName) ||
			(projectIndex && projectIndex.name) ||
			'project',
		),
		deterministic: Boolean(runConfig.deterministic) || (Number.isFinite(fixedTimestep) && fixedTimestep > 0),
	}
}

function resetActiveCaptureSession(project) {
	captureState.project = project
	captureState.session = {
		projectKey: project.projectKey,
		deterministic: project.deterministic,
		runTimestamp: formatCaptureTimestamp(),
		targetPromise: null,
	}
	scheduleCaptureCompareRefresh()
}

async function fileExists(directoryHandle, filename) {
	try {
		await directoryHandle.getFileHandle(filename)
		return true
	} catch (error) {
		if (error && error.name === 'NotFoundError') {
			return false
		}
		throw error
	}
}

async function getOptionalDirectoryHandle(parentHandle, name) {
	try {
		return await parentHandle.getDirectoryHandle(name)
	} catch (error) {
		if (error && error.name === 'NotFoundError') {
			return null
		}
		throw error
	}
}

async function ensureCaptureTarget() {
	if (!captureState.rootHandle || !captureState.session) {
		return null
	}
	if (!captureState.session.targetPromise) {
		captureState.session.targetPromise = (async () => {
			const snapshotsDir = await captureState.rootHandle.getDirectoryHandle('spx-snapshots', { create: true })
			const projectDir = await snapshotsDir.getDirectoryHandle(captureState.session.projectKey, { create: true })
			if (!captureState.session.deterministic) {
				const runsDir = await projectDir.getDirectoryHandle('runs', { create: true })
				const runDir = await runsDir.getDirectoryHandle(captureState.session.runTimestamp, { create: true })
				return { directory: runDir, destination: `runs/${captureState.session.runTimestamp}` }
			}

			const hasBaseline = await fileExists(projectDir, '.baseline.ready')
			if (!hasBaseline) {
				const baselineDir = await projectDir.getDirectoryHandle('baseline', { create: true })
				return { directory: baselineDir, destination: 'baseline', projectDir, writeBaselineMarker: true }
			}

			const runsDir = await projectDir.getDirectoryHandle('runs', { create: true })
			const runDir = await runsDir.getDirectoryHandle(captureState.session.runTimestamp, { create: true })
			return { directory: runDir, destination: `runs/${captureState.session.runTimestamp}` }
		})()
	}
	return captureState.session.targetPromise
}

async function markBaselineReady(projectDir) {
	const marker = await projectDir.getFileHandle('.baseline.ready', { create: true })
	const writer = await marker.createWritable()
	await writer.write(JSON.stringify({
		createdAt: new Date().toISOString(),
		projectKey: captureState.session ? captureState.session.projectKey : '',
	}, null, 2))
	await writer.close()
}

function buildCaptureFilename(request) {
	return request.filename
}

function formatCaptureFileMeta(file) {
	const sizeKB = (file.size / 1024).toFixed(1)
	const modifiedAt = new Date(file.lastModified).toLocaleString()
	return `${sizeKB} KB · ${modifiedAt}`
}

async function listCaptureFiles(directoryHandle) {
	if (!directoryHandle) {
		return new Map()
	}
	const entries = []
	for await (const entry of directoryHandle.values()) {
		if (entry.kind !== 'file' || !entry.name.toLowerCase().endsWith('.png')) {
			continue
		}
		entries.push(entry)
	}
	entries.sort((left, right) => left.name.localeCompare(right.name))

	const files = new Map()
	for (const entry of entries) {
		const file = await entry.getFile()
		const url = URL.createObjectURL(file)
		captureState.previewUrls.push(url)
		files.set(entry.name, {
			name: entry.name,
			file,
			url,
			meta: formatCaptureFileMeta(file),
		})
	}
	return files
}

async function findLatestRunDirectory(projectDir) {
	const runsDir = await getOptionalDirectoryHandle(projectDir, 'runs')
	if (!runsDir) {
		return null
	}
	let latestRun = null
	for await (const entry of runsDir.values()) {
		if (entry.kind !== 'directory') {
			continue
		}
		if (!latestRun || entry.name > latestRun.name) {
			latestRun = entry
		}
	}
	return latestRun
}

function formatCaptureSimilarityPercent(score) {
	return `${(score * 100).toFixed(2)}%`
}

function getCaptureSimilarityTone(score) {
	if (score >= 0.995) {
		return 'capture-compare-score-strong'
	}
	if (score >= 0.97) {
		return 'capture-compare-score-good'
	}
	if (score >= 0.9) {
		return 'capture-compare-score-warn'
	}
	return 'capture-compare-score-bad'
}

function formatCaptureBitmapSize(width, height) {
	return `${width}x${height}`
}

const captureQuickCompareSize = 128
const captureDetailedCompareSize = 320
const captureDetailedCompareThreshold = 0.985

async function loadCaptureBitmap(file) {
	if (typeof createImageBitmap === 'function') {
		return createImageBitmap(file)
	}
	return await new Promise((resolve, reject) => {
		const url = URL.createObjectURL(file)
		const image = new Image()
		image.onload = () => {
			URL.revokeObjectURL(url)
			resolve(image)
		}
		image.onerror = () => {
			URL.revokeObjectURL(url)
			reject(new Error(`Failed to decode ${file.name}`))
		}
		image.src = url
	})
}

function readCaptureImageData(imageSource, width, height) {
	const canvas = document.createElement('canvas')
	canvas.width = width
	canvas.height = height
	const context = canvas.getContext('2d', { willReadFrequently: true })
	context.drawImage(imageSource, 0, 0, width, height)
	return context.getImageData(0, 0, width, height).data
}

function readCaptureGrayscalePixels(imageSource, width, height) {
	const rgbaPixels = readCaptureImageData(imageSource, width, height)
	const grayscalePixels = new Uint8Array(rgbaPixels.length / 4)
	for (let src = 0, dest = 0; src < rgbaPixels.length; src += 4, dest++) {
		grayscalePixels[dest] = (
			rgbaPixels[src] * 77 +
			rgbaPixels[src + 1] * 150 +
			rgbaPixels[src + 2] * 29
		) >> 8
	}
	return grayscalePixels
}

function computeCaptureNormalizedDiff(leftPixels, rightPixels) {
	const pixelCount = Math.min(leftPixels.length, rightPixels.length)
	if (pixelCount === 0) {
		return 1
	}

	let totalDiff = 0
	for (let i = 0; i < pixelCount; i++) {
		totalDiff += Math.abs(leftPixels[i] - rightPixels[i])
	}
	return totalDiff / (pixelCount * 255)
}

function buildCaptureCompareSize(leftBitmap, rightBitmap, limit) {
	return {
		width: Math.max(1, Math.min(limit, leftBitmap.width, rightBitmap.width)),
		height: Math.max(1, Math.min(limit, leftBitmap.height, rightBitmap.height)),
	}
}

function buildCaptureSimilarityResult(score, detail) {
	return {
		status: 'ready',
		score,
		badgeClass: `capture-compare-score ${getCaptureSimilarityTone(score)}`,
		badgeText: `${formatCaptureSimilarityPercent(score)} match`,
		detail,
	}
}

async function computeCaptureSimilarity(leftRecord, rightRecord) {
	if (!leftRecord || !rightRecord) {
		if (leftRecord || rightRecord) {
			return {
				status: 'missing',
				badgeClass: 'capture-compare-score capture-compare-score-missing',
				badgeText: leftRecord ? 'Missing run' : 'Missing baseline',
				detail: '',
				score: null,
			}
		}
		return {
			status: 'missing',
			badgeClass: 'capture-compare-score capture-compare-score-missing',
			badgeText: 'No pair',
			detail: '',
			score: null,
		}
	}

	const leftBitmap = await loadCaptureBitmap(leftRecord.file)
	const rightBitmap = await loadCaptureBitmap(rightRecord.file)
	try {
		const leftSize = formatCaptureBitmapSize(leftBitmap.width, leftBitmap.height)
		const rightSize = formatCaptureBitmapSize(rightBitmap.width, rightBitmap.height)
		const sizeNote = leftSize === rightSize
			? ''
			: `Size mismatch: ${leftSize} vs ${rightSize}; `

		const quickSize = buildCaptureCompareSize(leftBitmap, rightBitmap, captureQuickCompareSize)
		const quickLeftPixels = readCaptureGrayscalePixels(leftBitmap, quickSize.width, quickSize.height)
		const quickRightPixels = readCaptureGrayscalePixels(rightBitmap, quickSize.width, quickSize.height)
		const quickScore = Math.max(0, 1 - computeCaptureNormalizedDiff(quickLeftPixels, quickRightPixels))
		const quickComparedSize = formatCaptureBitmapSize(quickSize.width, quickSize.height)

		if (quickScore >= captureDetailedCompareThreshold && leftSize === rightSize) {
			return buildCaptureSimilarityResult(
				quickScore,
				`${sizeNote}Quick grayscale compare at ${quickComparedSize}`,
			)
		}

		const detailedSize = buildCaptureCompareSize(leftBitmap, rightBitmap, captureDetailedCompareSize)
		const detailedLeftPixels = readCaptureImageData(leftBitmap, detailedSize.width, detailedSize.height)
		const detailedRightPixels = readCaptureImageData(rightBitmap, detailedSize.width, detailedSize.height)
		const detailedScore = Math.max(0, 1 - computeCaptureNormalizedDiff(detailedLeftPixels, detailedRightPixels))
		const detailedComparedSize = formatCaptureBitmapSize(detailedSize.width, detailedSize.height)

		return buildCaptureSimilarityResult(
			detailedScore,
			`${sizeNote}Quick ${formatCaptureSimilarityPercent(quickScore)} at ${quickComparedSize}; refined RGBA at ${detailedComparedSize}`,
		)
	} catch (error) {
		return {
			status: 'error',
			score: null,
			badgeClass: 'capture-compare-score capture-compare-score-missing',
			badgeText: 'Compare failed',
			detail: error && error.message ? error.message : 'Unknown image compare error',
		}
	} finally {
		if (typeof leftBitmap.close === 'function') {
			leftBitmap.close()
		}
		if (typeof rightBitmap.close === 'function') {
			rightBitmap.close()
		}
	}
}

async function buildCaptureComparisons(names, baselineFiles, runFiles) {
	return await Promise.all(names.map(async (name) => ({
		name,
		baseline: baselineFiles.get(name),
		run: runFiles.get(name),
		similarity: await computeCaptureSimilarity(baselineFiles.get(name), runFiles.get(name)),
	})))
}

function createCaptureCompareFileCell(name, similarity) {
	const cell = document.createElement('div')
	cell.className = 'capture-compare-file'

	const title = document.createElement('div')
	title.className = 'capture-compare-name'
	title.textContent = name
	cell.appendChild(title)

	const badge = document.createElement('div')
	badge.className = similarity.badgeClass
	badge.textContent = similarity.badgeText
	cell.appendChild(badge)

	if (similarity.detail) {
		const detail = document.createElement('div')
		detail.className = 'capture-compare-detail'
		detail.textContent = similarity.detail
		cell.appendChild(detail)
	}

	return cell
}

function createCaptureCompareCell(label, imageRecord) {
	const cell = document.createElement('div')
	cell.className = 'capture-compare-cell'

	const title = document.createElement('p')
	title.className = 'capture-compare-label'
	title.textContent = label
	cell.appendChild(title)

	if (!imageRecord) {
		const missing = document.createElement('div')
		missing.className = 'capture-compare-missing'
		missing.textContent = 'No image'
		cell.appendChild(missing)
		return cell
	}

	const link = document.createElement('a')
	link.className = 'capture-compare-link'
	link.href = imageRecord.url
	link.target = '_blank'
	link.rel = 'noopener noreferrer'

	const image = document.createElement('img')
	image.className = 'capture-compare-image'
	image.src = imageRecord.url
	image.alt = imageRecord.name
	image.loading = 'lazy'

	link.appendChild(image)
	cell.appendChild(link)

	const meta = document.createElement('div')
	meta.className = 'capture-compare-meta'
	meta.textContent = imageRecord.meta
	cell.appendChild(meta)

	return cell
}

function renderCaptureCompare(projectKey, runLabel, comparisons) {
	const grid = document.getElementById('captureCompareGrid')
	const summary = document.getElementById('captureCompareSummary')
	grid.innerHTML = ''

	if (comparisons.length === 0) {
		setCaptureCompareEmpty('No capture images found yet.', `${projectKey} · waiting for baseline or run`)
		return
	}

	const matchedComparisons = comparisons.filter((item) => item.similarity.status === 'ready')
	const incompleteCount = comparisons.length - matchedComparisons.length
	const averageSimilarity = matchedComparisons.length > 0
		? matchedComparisons.reduce((sum, item) => sum + item.similarity.score, 0) / matchedComparisons.length
		: null

	const summaryParts = [
		`${projectKey}`,
		`baseline vs ${runLabel}`,
		`${comparisons.length} frame(s)`,
	]
	if (averageSimilarity !== null) {
		summaryParts.push(`avg ${formatCaptureSimilarityPercent(averageSimilarity)}`)
	}
	if (incompleteCount > 0) {
		summaryParts.push(`${incompleteCount} incomplete`)
	}
	summary.textContent = summaryParts.join(' · ')

	for (const item of comparisons) {
		const row = document.createElement('div')
		row.className = 'capture-compare-row'

		row.appendChild(createCaptureCompareFileCell(item.name, item.similarity))
		row.appendChild(createCaptureCompareCell('Baseline', item.baseline))
		row.appendChild(createCaptureCompareCell(runLabel, item.run))
		grid.appendChild(row)
	}
}

async function refreshCaptureCompare() {
	if (!captureState.rootHandle) {
		setCaptureCompareEmpty('Choose a capture folder to browse baseline and run images.')
		return
	}
	if (!captureState.project) {
		setCaptureCompareEmpty('Start a project to load capture metadata.')
		return
	}

	const snapshotsDir = await getOptionalDirectoryHandle(captureState.rootHandle, 'spx-snapshots')
	if (!snapshotsDir) {
		setCaptureCompareEmpty('No spx-snapshots folder found in the selected directory.', `${captureState.project.projectKey} · no snapshots yet`)
		return
	}

	const projectDir = await getOptionalDirectoryHandle(snapshotsDir, captureState.project.projectKey)
	if (!projectDir) {
		setCaptureCompareEmpty(`No snapshots yet for ${captureState.project.projectKey}.`, `${captureState.project.projectKey} · waiting for first capture`)
		return
	}

	const baselineDir = await getOptionalDirectoryHandle(projectDir, 'baseline')
	let runLabel = 'latest run'
	let runDir = null

	const runsDir = await getOptionalDirectoryHandle(projectDir, 'runs')
	if (runsDir && captureState.session) {
		runDir = await getOptionalDirectoryHandle(runsDir, captureState.session.runTimestamp)
		if (runDir) {
			runLabel = `run ${captureState.session.runTimestamp}`
		}
	}
	if (!runDir) {
		const latestRunDir = await findLatestRunDirectory(projectDir)
		if (latestRunDir) {
			runDir = latestRunDir
			runLabel = `run ${latestRunDir.name}`
		}
	}

	revokeCapturePreviewUrls()
	const baselineFiles = await listCaptureFiles(baselineDir)
	const runFiles = await listCaptureFiles(runDir)
	const names = Array.from(new Set([
		...baselineFiles.keys(),
		...runFiles.keys(),
	])).sort((left, right) => left.localeCompare(right))
	const comparisons = await buildCaptureComparisons(names, baselineFiles, runFiles)
	renderCaptureCompare(captureState.project.projectKey, runLabel, comparisons)
}

function createFileSystemCaptureHost() {
	return {
		async handleCapture(request, blob) {
			const target = await ensureCaptureTarget()
			if (!target) {
				throw new Error('Capture directory is not selected')
			}
			const filename = buildCaptureFilename(request)
			const file = await target.directory.getFileHandle(filename, { create: true })
			const writer = await file.createWritable()
			await writer.write(blob)
			await writer.close()
			if (target.writeBaselineMarker && target.projectDir) {
				await markBaselineReady(target.projectDir)
				target.writeBaselineMarker = false
			}
			console.log('[spx capture saved]', {
				projectKey: captureState.session ? captureState.session.projectKey : '',
				destination: target.destination,
				filename,
				request,
			})
			scheduleCaptureCompareRefresh()
		},
	}
}

function installCaptureHostIfReady() {
	if (typeof runnerWindow.spxSetCaptureHost !== 'function') {
		return false
	}
	if (!captureState.rootHandle) {
		runnerWindow.spxSetCaptureHost(null)
		return true
	}
	runnerWindow.spxSetCaptureHost(createFileSystemCaptureHost())
	return true
}

async function selectCaptureDirectory() {
	if (!browserSupportsCaptureDirectory()) {
		updateCaptureStatus('Output · Folder access unavailable', true)
		return
	}
	try {
		captureState.rootHandle = await window.showDirectoryPicker(buildCaptureDirectoryPickerOptions())
		const handleName = captureState.rootHandle && captureState.rootHandle.name ? captureState.rootHandle.name : 'selected'
		updateCaptureStatus(`Output · ${handleName}`)
		installCaptureHostIfReady()
		scheduleCaptureCompareRefresh()
	} catch (error) {
		if (error && error.name === 'AbortError') {
			updateCaptureStatus('Output · Choose Downloads, Desktop, or another regular folder')
			return
		}
		console.error('Failed to select capture directory', error)
		updateCaptureStatus('Output · Selection failed; avoid system folders', true)
	}
}

function onProgress(value) {
	document.getElementById('progress-bar').value = value;
	if (value >= 1) {
		document.getElementById('tab-game').style.display = 'block';
		document.getElementById('tab-loader').style.display = 'none';
	}
	document.getElementById('progress-bar').style.display = value >= 1 ? 'none' : 'block';
}

async function initEngine() {
	await runnerWindow.initEngine(null, { logLevel: LOG_LEVEL_VERBOSE })
	installCaptureHostIfReady()
}

async function startGame() {
	await startProject('/game.zip')
}

async function stopGame() {
	await runnerWindow.stopGame()
}

async function pauseGame() {
	if (runnerWindow.pauseGame) {
		runnerWindow.pauseGame();
	} else {
		console.warn("Pause function not available");
	}
}

async function resumeGame() {
	if (runnerWindow.resumeGame) {
		runnerWindow.resumeGame();
	} else {
		console.warn("Resume function not available");
	}
}

async function stepNextFrame() {
	if (runnerWindow.stepNextFrame) {
		runnerWindow.stepNextFrame();
	} else {
		console.warn("StepNextFrame function not available");
	}
}
async function toggleSizeFrame() {
	const SMALL_SIZE = { width: 480, height: 360 };
	const LARGE_SIZE = { width: 960, height: 720 };

	const size = enlarged ? SMALL_SIZE : LARGE_SIZE;
	iframe.width = size.width;
	iframe.height = size.height;
	enlarged = !enlarged;
}
const LOG_LEVEL_VERBOSE = 0
let stopGameFlag = true

async function startProject(zipUrl) {
	if(stopGameFlag === false){
		console.error("game is running.")
		return
	}

	runnerWindow.addEventListener('onProgress', (event) => {
		onProgress(event.detail.progress)
	})
	runnerWindow.onGameError(function (msg) {
		console.error("onGameError", msg)
	})
	runnerWindow.onGameExit(function (code) {
		stopGameFlag = true
		console.error("onGameExit", code)
	})
	runnerWindow.onEngineCrash(function (code) {
		stopGameFlag = true
		console.error("onEngineCrash", code)
	})

	runnerWindow.spxIsForceDebugLog = true

	let zipped = await (await (fetch(zipUrl))).arrayBuffer()
	let unzipped = fflate.unzipSync(new Uint8Array(zipped))
	resetActiveCaptureSession(parseCaptureProject(unzipped))
	configureAIInteraction(unzipped)
	if (captureState.rootHandle) {
		updateCaptureStatus(`Output · ${captureState.rootHandle.name} / ${captureState.session.projectKey}`)
	}
	let files = {}
	Object.entries(unzipped).forEach(([path, data]) => {
		if (path.endsWith('/')) return // skip directories
		files[path] = { lastModified: Date.now(), content: data.buffer }
	})
	await runnerWindow.initGame(files)
	await runnerWindow.startGame()
	stopGameFlag = false
}

function resolveAIEndpointParam(value) {
	if (!value) return null
	try {
		const resolvedAIEndpoint = new URL(value, window.location.href)
		if (!['http:', 'https:'].includes(resolvedAIEndpoint.protocol)) {
			console.warn('Ignoring aiEndpoint query parameter with unsupported protocol', value)
			return null
		}
		if (resolvedAIEndpoint.origin !== window.location.origin) {
			console.warn('Ignoring cross-origin aiEndpoint query parameter', value)
			return null
		}
		return `${resolvedAIEndpoint.pathname}${resolvedAIEndpoint.search}${resolvedAIEndpoint.hash}`
	} catch (error) {
		console.warn('Ignoring invalid aiEndpoint query parameter', value, error)
		return null
	}
}

function configureAIInteraction(unzipped) {
	const params = new URLSearchParams(window.location.search)
	const aiEndpoint = resolveAIEndpointParam(params.get('aiEndpoint'))
	if (runnerWindow.xbuilder_set_ai_interaction_api_endpoint) {
		runnerWindow.xbuilder_set_ai_interaction_api_endpoint(aiEndpoint || '')
	}

	const descriptionFile = unzipped['builder-ai-description.md'] || unzipped['builder-ai-description']
	if (runnerWindow.xbuilder_set_ai_description) {
		const description = descriptionFile ? new TextDecoder('utf-8').decode(descriptionFile) : ''
		runnerWindow.xbuilder_set_ai_description(description)
	}
}
