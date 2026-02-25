package com.comradwatch.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.comradwatch.android.ui.ComradNavigation
import com.comradwatch.android.ui.theme.ComradTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            ComradTheme {
                ComradNavigation()
            }
        }
    }
}
