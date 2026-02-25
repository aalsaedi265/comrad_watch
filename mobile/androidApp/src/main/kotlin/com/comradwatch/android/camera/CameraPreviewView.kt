package com.comradwatch.android.camera

import android.content.Context
import android.util.Log
import android.widget.FrameLayout
import com.pedro.common.ConnectChecker
import com.pedro.library.rtmp.RtmpCamera2
import com.pedro.library.view.OpenGlView

/**
 * Camera preview + RTMP streaming wrapper.
 *
 * Simple API:
 *   view.startStreaming(rtmpUrl)
 *   view.stopStreaming()
 *
 * Uses RootEncoder's OpenGlView for preview rendering
 * and RtmpCamera2 for RTMP streaming.
 */
class CameraPreviewView(context: Context) : FrameLayout(context), ConnectChecker {

    private val openGlView: OpenGlView = OpenGlView(context)
    private var rtmpCamera: RtmpCamera2
    private var pendingRtmpUrl: String? = null
    var onConnectionChange: ((StreamStatus) -> Unit)? = null

    enum class StreamStatus {
        CONNECTED, DISCONNECTED, FAILED, AUTH_ERROR
    }

    init {
        // Add OpenGlView as the full-size child view
        addView(openGlView, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))

        // Create camera with the OpenGlView for preview rendering
        rtmpCamera = RtmpCamera2(openGlView, this)
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        // Start camera preview when the view is attached
        rtmpCamera.startPreview()

        // If streaming was requested before the view was ready
        pendingRtmpUrl?.let { url ->
            pendingRtmpUrl = null
            startStreaming(url)
        }
    }

    override fun onDetachedFromWindow() {
        stopStreaming()
        super.onDetachedFromWindow()
    }

    /**
     * Start RTMP streaming to the given URL.
     * Format: rtmp://server:1935/live/STREAM_KEY
     */
    fun startStreaming(rtmpUrl: String) {
        if (!rtmpCamera.isOnPreview) {
            // View not ready yet — queue the request
            pendingRtmpUrl = rtmpUrl
            return
        }

        if (rtmpCamera.isStreaming) return

        // Prepare audio: 128kbps, 44.1kHz, stereo
        val audioReady = rtmpCamera.prepareAudio(128000, 44100, true)

        // Prepare video: 720p, 30fps, 2.5Mbps, rotation 0
        val videoReady = rtmpCamera.prepareVideo(1280, 720, 30, 2_500_000, 0)

        if (audioReady && videoReady) {
            rtmpCamera.startStream(rtmpUrl)
            Log.i(TAG, "RTMP stream started: $rtmpUrl")
        } else {
            Log.e(TAG, "Failed to prepare: audio=$audioReady video=$videoReady")
            onConnectionChange?.invoke(StreamStatus.FAILED)
        }
    }

    /** Stop streaming and release camera. */
    fun stopStreaming() {
        if (rtmpCamera.isStreaming) {
            rtmpCamera.stopStream()
        }
        if (rtmpCamera.isOnPreview) {
            rtmpCamera.stopPreview()
        }
        Log.i(TAG, "Stream and preview stopped")
    }

    /** Switch between front and back camera. */
    fun switchCamera() {
        rtmpCamera.switchCamera()
    }

    // --- ConnectChecker callbacks ---

    override fun onConnectionStarted(url: String) {
        Log.i(TAG, "RTMP connecting to: $url")
    }

    override fun onConnectionSuccess() {
        Log.i(TAG, "RTMP connected")
        post { onConnectionChange?.invoke(StreamStatus.CONNECTED) }
    }

    override fun onConnectionFailed(reason: String) {
        Log.e(TAG, "RTMP connection failed: $reason")
        post { onConnectionChange?.invoke(StreamStatus.FAILED) }
    }

    override fun onNewBitrate(bitrate: Long) {
        // Could adjust quality adaptively in the future
    }

    override fun onDisconnect() {
        Log.w(TAG, "RTMP disconnected")
        post { onConnectionChange?.invoke(StreamStatus.DISCONNECTED) }
    }

    override fun onAuthError() {
        Log.e(TAG, "RTMP auth error")
        post { onConnectionChange?.invoke(StreamStatus.AUTH_ERROR) }
    }

    override fun onAuthSuccess() {
        Log.i(TAG, "RTMP auth success")
    }

    companion object {
        private const val TAG = "CameraPreview"
    }
}
