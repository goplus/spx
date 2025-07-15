// 视频录制器类
class VideoRecorder {
    constructor(canvas, logger) {
        this.canvas = canvas;
        this.logger = logger;
        this.stream = null;
        this.recorder = null;
        this.chunks = [];
        this.isRecording = false;
        this.isInitialized = false;
        this.ffmpeg = null;
        this.ffmpegInitialized = false;

        // 录制时间跟踪
        this.recordingStartTime = null;
        this.recordingEndTime = null;
        this.verboseLog = false;
    }

    logVerbose(message) {
        if (this.verboseLog) {
            this.logger(message);
        }
    }

    async init() {
        try {
            // 获取Canvas流
            this.stream = this.canvas.captureStream(30);
            this.logger('Canvas流获取成功，FPS: 30');

            // 创建MediaRecorder
            this.recorder = new MediaRecorder(this.stream, {
                mimeType: 'video/webm;codecs=vp9'
            });

            // 设置事件监听
            this.recorder.ondataavailable = e => {
                this.logVerbose(`数据块收到: ${e.data.size} bytes`);
                if (e.data.size > 0) {
                    this.chunks.push(e.data);
                }
            };

            this.recorder.onstart = () => {
                this.logger('录制开始');
                updateStatus('正在录制...', 'recording');
            };

            this.recorder.onstop = () => {
                this.logger('录制停止');
                updateStatus('录制完成', 'success');
                this.processRecording();
            };

            this.recorder.onerror = (e) => {
                this.logger('录制错误: ' + e.error);
                updateStatus('录制错误', 'ready');
            };

            this.isInitialized = true;
            this.logger('VideoRecorder初始化成功');
            return true;

        } catch (error) {
            this.logger('初始化录制器失败: ' + error.message);
            updateStatus('录制器初始化失败', 'ready');
            return false;
        }
    }

    async initFFmpeg() {
        // 如果已经初始化过，直接返回
        if (this.ffmpegInitialized) {
            return true;
        }

        try {
            // 检查全局变量是否存在
            this.logger('检查FFmpeg.wasm库加载状态...');
            console.log('FFmpegWASM:', typeof FFmpegWASM);
            console.log('FFmpegUtil:', typeof FFmpegUtil);

            if (typeof FFmpegWASM === 'undefined') {
                throw new Error('FFmpegWASM未定义，请检查ffmpeg.js是否正确加载');
            }

            if (typeof FFmpegUtil === 'undefined') {
                throw new Error('FFmpegUtil未定义，请检查util/index.js是否正确加载');
            }

            this.logger('开始初始化FFmpeg...');
            this.ffmpeg = new FFmpegWASM.FFmpeg();

            // 转码状态跟踪
            this.transcodeStartTime = null;
            this.lastFrameCount = 0;
            this.totalFrames = null;
            this.videoDuration = null;

            // 设置FFmpeg事件监听器
            this.ffmpeg.on("log", ({ message }) => {
                this.logVerbose('FFmpeg日志:', message);
                // 只在转码阶段处理进度
                // 转码阶段 - 提取当前帧数并计算百分比
                const frameMatch = message.match(/frame=\s*(\d+)/);
                if (frameMatch) {
                    const currentFrame = parseInt(frameMatch[1]);
                    this.updateTranscodeProgressFromFrame(currentFrame);
                }
            });

            this.ffmpeg.on("progress", ({ progress, time }) => {
                // 只使用time值，忽略不可靠的progress值
                if (typeof time === 'number' && isFinite(time) && time >= 0) {
                    const seconds = time / 1000000; // 转换为秒
                    this.updateTranscodeTime(seconds);
                }
            });

            // 加载FFmpeg核心
            this.logger('加载FFmpeg核心文件...');
            const baseURL = window.location.origin;
            const coreURL = `${baseURL}/ffmpeg/core/package/dist/umd/ffmpeg-core.js`;

            await this.ffmpeg.load({
                coreURL: coreURL,
            });

            this.ffmpegInitialized = true;
            this.logger('FFmpeg初始化成功');
            return true;

        } catch (error) {
            this.logger('FFmpeg初始化失败: ' + error.message);
            console.error('FFmpeg初始化详细错误:', error);
            this.ffmpegInitialized = false;
            return false;
        }
    }

    updateTranscodeProgressFromFrame(frameCount) {
        if (!this.transcodeStartTime) {
            this.transcodeStartTime = Date.now();
        }

        this.lastFrameCount = frameCount;
        const elapsed = (Date.now() - this.transcodeStartTime) / 1000;

        // 计算基于帧数的百分比
        let percentage = 0;
        if (this.totalFrames && this.totalFrames > 0) {
            percentage = Math.min(Math.round((frameCount / this.totalFrames) * 100), 100);
        }

        // 显示基于frame的进度信息
        this.showTranscodeProgress(frameCount, elapsed, null, percentage);
    }

    updateTranscodeTime(seconds) {
        if (!this.transcodeStartTime) {
            this.transcodeStartTime = Date.now();
        }

        const elapsed = (Date.now() - this.transcodeStartTime) / 1000;

        // 计算基于帧数的百分比
        let percentage = 0;
        if (this.totalFrames && this.totalFrames > 0 && this.lastFrameCount > 0) {
            percentage = Math.min(Math.round((this.lastFrameCount / this.totalFrames) * 100), 100);
        }

        this.showTranscodeProgress(this.lastFrameCount, elapsed, seconds, percentage);
    }

    showTranscodeProgress(frameCount, elapsed, videoTime = null, percentage = null) {
        const transcodeSection = document.getElementById('transcode-section');
        const transcodeMessage = document.getElementById('transcode-message');
        const transcodeProgressFill = document.getElementById('transcode-progress-fill');
        const transcodeTimeInfo = document.getElementById('transcode-time-info');

        if (transcodeSection && transcodeMessage && transcodeProgressFill && transcodeTimeInfo) {
            transcodeSection.style.display = 'block';

            // 显示进度信息
            if (percentage !== null && percentage > 0) {
                // 有百分比信息时显示准确进度
                transcodeMessage.textContent = `转码进行中... ${percentage}% (帧: ${frameCount}/${this.totalFrames || '?'})`;
                transcodeProgressFill.style.width = `${percentage}%`;
            } else {
                // 没有百分比信息时显示动态进度
                const dots = '.'.repeat((Math.floor(elapsed) % 3) + 1);
                transcodeMessage.textContent = `转码进行中${dots} (帧: ${frameCount})`;
                const animationPercent = (elapsed * 10) % 100;
                transcodeProgressFill.style.width = `${Math.min(animationPercent, 90)}%`;
            }

            let timeInfo = `处理时间: ${Math.round(elapsed)}s`;
            if (videoTime) {
                timeInfo += ` | 视频时长: ${Math.round(videoTime)}s`;
            }
            transcodeTimeInfo.textContent = timeInfo;
        }
    }

    updateTranscodeProgress(percent, seconds) {
        // 保留这个方法以保持兼容性，但现在只是一个空方法
        // 所有进度更新都通过新的方法处理
    }

    hideTranscodeProgress() {
        const transcodeSection = document.getElementById('transcode-section');
        if (transcodeSection) {
            transcodeSection.style.display = 'none';
        }
    }

    start() {
        if (!this.isInitialized || !this.recorder || this.isRecording) {
            this.logger('录制器未初始化或已在录制中');
            return false;
        }

        try {
            this.chunks = [];
            this.recordingStartTime = Date.now(); // 记录开始时间
            this.recorder.start(10); // 每10ms收集一次数据
            this.isRecording = true;
            this.logger('录制开始');
            return true;

        } catch (error) {
            this.logger('开始录制失败: ' + error.message);
            return false;
        }
    }

    stop() {
        if (!this.isRecording || !this.recorder) {
            this.logger('录制器未在录制中');
            return false;
        }

        try {
            this.recordingEndTime = Date.now(); // 记录结束时间
            this.recorder.stop();
            this.isRecording = false;
            this.logger('录制停止');
            return true;

        } catch (error) {
            this.logger('停止录制失败: ' + error.message);
            return false;
        }
    }

    async processRecording() {
        if (this.chunks.length === 0) {
            this.logger('没有录制数据');
            return;
        }

        try {
            // 创建预览
            this.createPreview();

            // 只下载WebM格式文件，不自动转换为MP4
            this.logger('录制完成，可以下载WebM格式文件');
            this.downloadWebM();

        } catch (error) {
            this.logger('处理录制失败: ' + error.message);
            this.downloadWebM();
        }
    }

    async transcodeToMP4() {
        try {
            const webmBlob = new Blob(this.chunks, { type: 'video/webm' });
            const inputFileName = 'input.webm';
            const outputFileName = 'output.mp4';

            this.logger('写入WebM文件到FFmpeg...');
            await this.ffmpeg.writeFile(inputFileName, await FFmpegUtil.fetchFile(webmBlob));

            // 获取视频信息，特别是总帧数
            this.logger('分析视频信息...');
            await this.getVideoInfo(inputFileName);

            this.logger('开始转码...');
            this.updateTranscodeProgress(0, 0);

            console.time('transcode');
            await this.ffmpeg.exec([
                '-i', inputFileName,
                '-c:v', 'libx264',
                '-preset', 'fast',
                '-crf', '23',
                '-movflags', '+faststart',
                outputFileName
            ]);
            console.timeEnd('transcode');

            this.logger('读取转码后的文件...');
            const mp4Data = await this.ffmpeg.readFile(outputFileName);

            // 清理临时文件
            await this.ffmpeg.deleteFile(inputFileName);
            await this.ffmpeg.deleteFile(outputFileName);

            this.hideTranscodeProgress();
            this.logger('转码完成');
            updateStatus('转码完成，准备下载...', 'success');

            return new Blob([mp4Data.buffer], { type: 'video/mp4' });

        } catch (error) {
            this.logger('转码失败: ' + error.message);
            this.hideTranscodeProgress();
            updateStatus('转码失败', 'ready');
            return null;
        }
    }

    async getVideoInfo(fileName) {
        try {
            // 重置转码状态
            this.transcodeStartTime = null;
            this.lastFrameCount = 0;
            this.totalFrames = 0;
            this.videoDuration = null;
            this.frameRate = 60; // 默认帧率，会被真实FPS覆盖

            this.logger('获取视频信息中...');
            // 首先尝试获取视频的真实信息（时长和FPS）
            await this.getBasicVideoInfo(fileName);
            const recordingDuration = this.getEstimatedRecordingDuration();
            // 最后的备用方案
            this.totalFrames = Math.round(recordingDuration * this.frameRate) || 300;
            this.logger(`备用方案计算帧数: ${this.totalFrames} (${recordingDuration.toFixed(2)}s × ${this.frameRate}fps)`);
        } catch (error) {
            this.logger('获取视频信息失败: ' + error.message);
            // 使用录制时间的最后备用方案
            const recordingDuration = this.getEstimatedRecordingDuration();
            this.totalFrames = Math.round(recordingDuration * this.frameRate) || 300;
            this.logger(`异常情况备用估算帧数: ${this.totalFrames}`);
        }
    }

    async getBasicVideoInfo(fileName) {

    }

    // 估算录制时长的辅助方法
    getEstimatedRecordingDuration() {
        const actualDuration = (this.recordingEndTime - this.recordingStartTime) / 1000;
        this.logger(`使用实际录制时间: ${actualDuration.toFixed(2)}s`);
        return actualDuration;
    }


    downloadMp4File(mp4Blob) {
        try {
            const downloadUrl = URL.createObjectURL(mp4Blob);
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = downloadUrl;
            link.download = `canvas-recording-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.mp4`;

            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);

            URL.revokeObjectURL(downloadUrl);

            const sizeMB = (mp4Blob.size / 1024 / 1024).toFixed(2);
            this.logger(`MP4下载完成，文件大小: ${sizeMB} MB`);
            updateStatus('MP4下载完成', 'success');

        } catch (error) {
            this.logger('MP4下载失败: ' + error.message);
        }
    }

    downloadWebM() {
        if (this.chunks.length === 0) {
            this.logger('没有录制数据可下载');
            return false;
        }

        try {
            const fullBlob = new Blob(this.chunks, { type: 'video/webm' });
            const downloadUrl = URL.createObjectURL(fullBlob);

            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = downloadUrl;
            link.download = `canvas-recording-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.webm`;

            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);

            URL.revokeObjectURL(downloadUrl);

            this.logger('WebM下载完成');
            updateStatus('WebM下载完成', 'success');
            return true;

        } catch (error) {
            this.logger('WebM下载失败: ' + error.message);
            return false;
        }
    }

    // 新增：按需下载MP4格式文件
    async downloadMp4() {
        if (this.chunks.length === 0) {
            this.logger('没有录制数据');
            return false;
        }

        try {
            // 延迟初始化FFmpeg
            if (!this.ffmpegInitialized) {
                this.logger('开始初始化FFmpeg...');
                updateStatus('初始化转码器...', 'recording');
                
                const initSuccess = await this.initFFmpeg();
                if (!initSuccess) {
                    this.logger('FFmpeg初始化失败，无法转换为MP4');
                    updateStatus('转码器初始化失败', 'ready');
                    return false;
                }
            }

            this.logger('开始转码为MP4格式...');
            updateStatus('转码中...', 'recording');

            const mp4Blob = await this.transcodeToMP4();
            if (mp4Blob) {
                this.downloadMp4File(mp4Blob);
                return true;
            } else {
                this.logger('转码失败');
                updateStatus('转码失败', 'ready');
                return false;
            }

        } catch (error) {
            this.logger('MP4转换失败: ' + error.message);
            updateStatus('转码失败', 'ready');
            return false;
        }
    }

    // 保持向后兼容的download方法
    download() {
        return this.downloadWebM();
    }

    createPreview() {
        if (this.chunks.length === 0) {
            this.logger('没有录制数据');
            return;
        }

        try {
            const fullBlob = new Blob(this.chunks, { type: 'video/webm' });
            const videoURL = URL.createObjectURL(fullBlob);

            const previewVideo = document.getElementById('previewVideo');
            previewVideo.src = videoURL;
            previewVideo.style.display = 'block';

            const sizeMB = (fullBlob.size / 1024 / 1024).toFixed(2);
            this.logger(`预览创建成功，文件大小: ${sizeMB} MB`);

        } catch (error) {
            this.logger('创建预览失败: ' + error.message);
        }
    }
} 