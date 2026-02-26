package com.comradwatch.android.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import com.comradwatch.android.ComradApp
import kotlinx.coroutines.launch

/**
 * Main screen — ONE button fills the entire screen.
 * Tap it → permissions check → start recording.
 *
 * Design principle: impossible to misuse.
 * No menus, no options, no confusion.
 * Small settings icon in the corner for setup access.
 */
@Composable
fun MainScreen(
    onStartRecording: () -> Unit,
    onOpenSettings: () -> Unit
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    // Google Drive connection state
    var driveConnected by remember { mutableStateOf<Boolean?>(null) }
    var driveLoading by remember { mutableStateOf(false) }

    // Check Google Drive status when screen appears and when returning from browser
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                scope.launch {
                    ComradApp.instance.api.getGoogleDriveStatus()
                        .onSuccess { driveConnected = it }
                }
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    // Pulsing animation on the button
    val infiniteTransition = rememberInfiniteTransition(label = "pulse")
    val scale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = 1.05f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200, easing = EaseInOutSine),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulse"
    )

    // Permission launcher
    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { permissions ->
        val allGranted = permissions.values.all { it }
        if (allGranted) {
            onStartRecording()
        }
    }

    fun checkAndStart() {
        val requiredPermissions = arrayOf(
            Manifest.permission.CAMERA,
            Manifest.permission.RECORD_AUDIO
        )
        val allGranted = requiredPermissions.all {
            ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED
        }
        if (allGranted) {
            onStartRecording()
        } else {
            permissionLauncher.launch(requiredPermissions)
        }
    }

    fun connectGoogleDrive() {
        driveLoading = true
        scope.launch {
            ComradApp.instance.api.getGoogleAuthUrl()
                .onSuccess { url ->
                    // Open Google OAuth in the default browser
                    val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url))
                    context.startActivity(intent)
                }
            driveLoading = false
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0A0A0A)),
        contentAlignment = Alignment.Center
    ) {
        // Main tap-to-record button — fills most of the screen
        Box(
            modifier = Modifier
                .size(280.dp)
                .scale(scale)
                .background(
                    color = MaterialTheme.colorScheme.primary,
                    shape = CircleShape
                )
                .clickable { checkAndStart() },
            contentAlignment = Alignment.Center
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = "⬤",
                    fontSize = 48.sp,
                    color = Color.White
                )
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "TAP TO\nRECORD",
                    fontSize = 28.sp,
                    fontWeight = FontWeight.Black,
                    color = Color.White,
                    textAlign = TextAlign.Center,
                    lineHeight = 34.sp
                )
            }
        }

        // Google Drive status — bottom left corner
        Box(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .padding(24.dp)
        ) {
            when (driveConnected) {
                true -> {
                    // Connected indicator
                    Text(
                        text = "Drive ✓",
                        fontSize = 12.sp,
                        color = Color(0xFF4CAF50),
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier
                            .background(Color(0xFF1A2E1A), RoundedCornerShape(8.dp))
                            .padding(horizontal = 12.dp, vertical = 6.dp)
                    )
                }
                false -> {
                    // Not connected — tappable
                    Text(
                        text = if (driveLoading) "Connecting..." else "Connect Drive",
                        fontSize = 12.sp,
                        color = Color(0xFFFF9800),
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier
                            .background(Color(0xFF2E2010), RoundedCornerShape(8.dp))
                            .clickable(enabled = !driveLoading) { connectGoogleDrive() }
                            .padding(horizontal = 12.dp, vertical = 6.dp)
                    )
                }
                null -> {
                    // Loading / unknown state — show nothing
                }
            }
        }

        // Small settings gear — bottom right corner
        IconButton(
            onClick = onOpenSettings,
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(24.dp)
        ) {
            Text(
                text = "⚙",
                fontSize = 24.sp,
                color = Color.Gray
            )
        }

        // App name — top center, subtle
        Text(
            text = "COMRAD WATCH",
            fontSize = 14.sp,
            fontWeight = FontWeight.Medium,
            color = Color(0xFF666666),
            modifier = Modifier
                .align(Alignment.TopCenter)
                .padding(top = 60.dp)
        )
    }
}
