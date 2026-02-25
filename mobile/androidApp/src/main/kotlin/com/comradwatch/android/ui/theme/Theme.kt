package com.comradwatch.android.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// High-contrast dark theme — optimized for quick recognition in stress.
private val DarkColors = darkColorScheme(
    primary = Color(0xFFFF3B30),       // Alert red — the main action color
    onPrimary = Color.White,
    secondary = Color(0xFF30D158),     // Safe green — "connected" indicator
    background = Color(0xFF0A0A0A),    // Near-black background
    onBackground = Color.White,
    surface = Color(0xFF1C1C1E),       // Dark surface for cards/menus
    onSurface = Color.White,
    error = Color(0xFFFF453A),
)

@Composable
fun ComradTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkColors,
        content = content
    )
}
