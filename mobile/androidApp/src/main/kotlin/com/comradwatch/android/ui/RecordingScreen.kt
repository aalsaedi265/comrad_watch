package com.comradwatch.android.ui

import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import com.comradwatch.android.ComradApp
import com.comradwatch.android.camera.CameraPreviewView
import com.comradwatch.android.service.StreamingService
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Recording screen — camera viewfinder with minimal UI.
 *
 * On entry:
 *   1. Calls API to get a stream key
 *   2. Starts foreground service (keeps app alive)
 *   3. Opens camera + starts RTMP streaming
 *
 * On stop:
 *   1. Stops RTMP stream
 *   2. Stops foreground service
 *   3. Navigates back
 */
@Composable
fun RecordingScreen(
    onStopAndSave: () -> Unit,
    onStopAndDiscard: () -> Unit
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    var showMenu by remember { mutableStateOf(false) }
    var elapsedSeconds by remember { mutableIntStateOf(0) }
    var isConnected by remember { mutableStateOf(false) }
    var connectionText by remember { mutableStateOf("CONNECTING...") }
    var cameraView by remember { mutableStateOf<CameraPreviewView?>(null) }

    // Timer
    LaunchedEffect(Unit) {
        while (true) {
            delay(1000)
            elapsedSeconds++
        }
    }

    // Start streaming when screen opens
    LaunchedEffect(Unit) {
        // 1. Get stream key from server
        val result = ComradApp.instance.api.startSession()
        result.onSuccess { session ->
            // 2. Start foreground service
            StreamingService.start(context)

            // 3. Start streaming (camera view will pick this up)
            cameraView?.startStreaming(session.rtmpUrl)
        }
        result.onFailure {
            connectionText = "SERVER ERROR"
        }
    }

    // Cleanup on exit
    DisposableEffect(Unit) {
        onDispose {
            cameraView?.stopStreaming()
            StreamingService.stop(context)
        }
    }

    // Pulsing red border
    val infiniteTransition = rememberInfiniteTransition(label = "border")
    val borderAlpha by infiniteTransition.animateFloat(
        initialValue = 0.4f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(800, easing = EaseInOutSine),
            repeatMode = RepeatMode.Reverse
        ),
        label = "border"
    )

    Box(modifier = Modifier.fillMaxSize()) {
        // Camera preview — fills entire screen
        AndroidView(
            factory = { ctx ->
                CameraPreviewView(ctx).also { view ->
                    view.onConnectionChange = { status ->
                        when (status) {
                            CameraPreviewView.StreamStatus.CONNECTED -> {
                                isConnected = true
                                connectionText = "LIVE"
                            }
                            CameraPreviewView.StreamStatus.DISCONNECTED -> {
                                isConnected = false
                                connectionText = "RECONNECTING..."
                            }
                            CameraPreviewView.StreamStatus.FAILED -> {
                                isConnected = false
                                connectionText = "FAILED"
                            }
                            CameraPreviewView.StreamStatus.AUTH_ERROR -> {
                                isConnected = false
                                connectionText = "AUTH ERROR"
                            }
                        }
                    }
                    cameraView = view
                }
            },
            modifier = Modifier.fillMaxSize()
        )

        // Pulsing red border overlay
        Box(
            modifier = Modifier
                .fillMaxSize()
                .border(
                    width = 4.dp,
                    color = Color.Red.copy(alpha = borderAlpha),
                    shape = RoundedCornerShape(0.dp)
                )
        )

        // Top bar: timer + connection status
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 50.dp, start = 16.dp, end = 16.dp)
                .background(Color.Black.copy(alpha = 0.5f), RoundedCornerShape(8.dp))
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Recording dot
            Box(
                modifier = Modifier
                    .size(12.dp)
                    .background(Color.Red, CircleShape)
            )
            Spacer(modifier = Modifier.width(8.dp))

            // Timer
            Text(
                text = formatTime(elapsedSeconds),
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = Color.White
            )

            Spacer(modifier = Modifier.weight(1f))

            // Connection status
            val statusColor = if (isConnected) Color(0xFF30D158) else Color(0xFFFF9500)
            Box(
                modifier = Modifier
                    .size(10.dp)
                    .background(statusColor, CircleShape)
            )
            Spacer(modifier = Modifier.width(6.dp))
            Text(
                text = connectionText,
                fontSize = 12.sp,
                fontWeight = FontWeight.Bold,
                color = statusColor
            )
        }

        // Menu toggle
        if (!showMenu) {
            Box(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 40.dp)
                    .background(Color.Black.copy(alpha = 0.6f), RoundedCornerShape(24.dp))
                    .clickable { showMenu = true }
                    .padding(horizontal = 24.dp, vertical = 12.dp)
            ) {
                Text("▲ MENU", fontSize = 14.sp, fontWeight = FontWeight.Bold, color = Color.White)
            }
        }

        // Stop menu overlay
        if (showMenu) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.7f))
                    .clickable { showMenu = false },
                contentAlignment = Alignment.Center
            ) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(16.dp)
                ) {
                    // STOP & SAVE
                    Button(
                        onClick = {
                            cameraView?.stopStreaming()
                            StreamingService.stop(context)
                            onStopAndSave()
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF30D158)),
                        modifier = Modifier.width(240.dp).height(64.dp),
                        shape = RoundedCornerShape(16.dp)
                    ) {
                        Text("STOP & SAVE", fontSize = 20.sp, fontWeight = FontWeight.Black, color = Color.White)
                    }

                    // STOP & DISCARD
                    Button(
                        onClick = {
                            cameraView?.stopStreaming()
                            StreamingService.stop(context)
                            onStopAndDiscard()
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF48484A)),
                        modifier = Modifier.width(240.dp).height(64.dp),
                        shape = RoundedCornerShape(16.dp)
                    ) {
                        Text("STOP & DISCARD", fontSize = 20.sp, fontWeight = FontWeight.Black, color = Color.White)
                    }

                    TextButton(onClick = { showMenu = false }) {
                        Text("CANCEL", fontSize = 16.sp, color = Color.Gray)
                    }
                }
            }
        }
    }
}

private fun formatTime(seconds: Int): String {
    val h = seconds / 3600
    val m = (seconds % 3600) / 60
    val s = seconds % 60
    return if (h > 0) String.format("%d:%02d:%02d", h, m, s)
    else String.format("%02d:%02d", m, s)
}
