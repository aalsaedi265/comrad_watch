package com.comradwatch.android.ui

import androidx.compose.runtime.Composable
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController

/**
 * App navigation — dead simple.
 *
 * Setup → Main → Recording
 *
 * Setup is shown once on first launch.
 * Main is the one-tap "start recording" screen.
 * Recording is the active camera + streaming screen.
 */
@Composable
fun ComradNavigation() {
    val navController = rememberNavController()

    NavHost(navController = navController, startDestination = "main") {
        composable("setup") {
            SetupScreen(onSetupComplete = {
                navController.navigate("main") {
                    popUpTo("setup") { inclusive = true }
                }
            })
        }
        composable("main") {
            MainScreen(
                onStartRecording = { navController.navigate("recording") },
                onOpenSettings = { navController.navigate("setup") }
            )
        }
        composable("recording") {
            RecordingScreen(
                onStopAndSave = { navController.popBackStack() },
                onStopAndDiscard = { navController.popBackStack() }
            )
        }
    }
}
