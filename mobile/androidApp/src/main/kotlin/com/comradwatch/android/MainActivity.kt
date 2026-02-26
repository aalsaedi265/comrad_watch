package com.comradwatch.android

import android.content.Intent
import android.os.Bundle
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.comradwatch.android.ui.ComradNavigation
import com.comradwatch.android.ui.handleInstagramCallback
import com.comradwatch.android.ui.theme.ComradTheme
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            ComradTheme {
                ComradNavigation()
            }
        }

        // Handle deep link if the activity was launched with one
        handleDeepLink(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleDeepLink(intent)
    }

    private fun handleDeepLink(intent: Intent?) {
        val uri = intent?.data ?: return

        // Handle Instagram OAuth callback: comradwatch://instagram-callback?code=...
        if (uri.scheme == "comradwatch" && uri.host == "instagram-callback") {
            val code = uri.getQueryParameter("code") ?: return
            Log.d("ComradWatch", "Received Instagram OAuth code")

            CoroutineScope(Dispatchers.IO).launch {
                handleInstagramCallback(code)
                    .onSuccess { username ->
                        Log.d("ComradWatch", "Instagram connected: $username")
                    }
                    .onFailure { error ->
                        Log.e("ComradWatch", "Instagram connect failed: ${error.message}")
                    }
            }
        }
    }
}
