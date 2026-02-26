package com.comradwatch.android.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.comradwatch.android.ComradApp
import kotlinx.coroutines.launch

/**
 * Setup screen — shown once on first launch.
 * Connects the user's account so the one-tap button works instantly.
 * Also allows connecting Instagram for automatic story posting.
 */
@Composable
fun SetupScreen(onSetupComplete: () -> Unit) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var serverUrl by remember { mutableStateOf("http://10.0.2.2:8080") }
    var isLoading by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }
    var isRegistering by remember { mutableStateOf(true) }
    var isLoggedIn by remember { mutableStateOf(false) }

    // Instagram state
    var igConnected by remember { mutableStateOf(false) }
    var igAccountId by remember { mutableStateOf<String?>(null) }
    var igLoading by remember { mutableStateOf(false) }
    var igError by remember { mutableStateOf<String?>(null) }

    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    // Fetch server config and Instagram status when logged in
    LaunchedEffect(isLoggedIn) {
        if (isLoggedIn) {
            // Fetch the Instagram App ID from the server
            ComradApp.instance.api.getConfig().onSuccess { config ->
                ComradApp.instance.instagramAppId = config.instagramAppId
            }
            // Check if the user already has Instagram connected
            ComradApp.instance.api.getInstagramStatus().onSuccess { status ->
                igConnected = status.connected
                igAccountId = status.accountId
            }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0A0A0A))
            .padding(24.dp)
            .verticalScroll(rememberScrollState()),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(
            text = "COMRAD WATCH",
            fontSize = 28.sp,
            fontWeight = FontWeight.Black,
            color = Color.White
        )
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            text = if (isLoggedIn) "Settings" else "One-time setup",
            fontSize = 16.sp,
            color = Color.Gray,
            textAlign = TextAlign.Center
        )
        Spacer(modifier = Modifier.height(40.dp))

        if (!isLoggedIn) {
            // --- Account Setup ---

            // Server URL
            OutlinedTextField(
                value = serverUrl,
                onValueChange = { serverUrl = it },
                label = { Text("Server URL") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                colors = OutlinedTextFieldDefaults.colors(
                    focusedTextColor = Color.White,
                    unfocusedTextColor = Color.White,
                    focusedBorderColor = MaterialTheme.colorScheme.primary,
                    unfocusedBorderColor = Color.Gray
                )
            )
            Spacer(modifier = Modifier.height(16.dp))

            // Email
            OutlinedTextField(
                value = email,
                onValueChange = { email = it },
                label = { Text("Email") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                colors = OutlinedTextFieldDefaults.colors(
                    focusedTextColor = Color.White,
                    unfocusedTextColor = Color.White,
                    focusedBorderColor = MaterialTheme.colorScheme.primary,
                    unfocusedBorderColor = Color.Gray
                )
            )
            Spacer(modifier = Modifier.height(16.dp))

            // Password
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("Password") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedTextColor = Color.White,
                    unfocusedTextColor = Color.White,
                    focusedBorderColor = MaterialTheme.colorScheme.primary,
                    unfocusedBorderColor = Color.Gray
                )
            )
            Spacer(modifier = Modifier.height(24.dp))

            // Error message
            errorMessage?.let {
                Text(
                    text = it,
                    color = MaterialTheme.colorScheme.error,
                    fontSize = 14.sp,
                    modifier = Modifier.padding(bottom = 16.dp)
                )
            }

            // Register / Login button
            Button(
                onClick = {
                    isLoading = true
                    errorMessage = null
                    ComradApp.instance.updateServerUrl(serverUrl)
                    scope.launch {
                        val result = if (isRegistering) {
                            ComradApp.instance.api.register(email, password)
                        } else {
                            ComradApp.instance.api.login(email, password)
                        }
                        isLoading = false
                        result.onSuccess { isLoggedIn = true }
                        result.onFailure { errorMessage = it.message }
                    }
                },
                enabled = !isLoading && email.isNotBlank() && password.isNotBlank(),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(56.dp),
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary
                )
            ) {
                if (isLoading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(24.dp),
                        color = Color.White
                    )
                } else {
                    Text(
                        text = if (isRegistering) "CREATE ACCOUNT" else "LOG IN",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // Toggle register/login
            TextButton(onClick = { isRegistering = !isRegistering }) {
                Text(
                    text = if (isRegistering) "Already have an account? Log in"
                           else "Need an account? Register",
                    color = Color.Gray
                )
            }
        } else {
            // --- Post-login settings (Instagram, etc.) ---

            // Instagram connection section
            Text(
                text = "INSTAGRAM",
                fontSize = 14.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFF666666),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 8.dp)
            )

            Text(
                text = "Connect your Instagram Business/Creator account to automatically post recordings as Stories.",
                fontSize = 14.sp,
                color = Color.Gray,
                modifier = Modifier.padding(bottom = 16.dp)
            )

            if (igConnected) {
                // Connected state
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(Color(0xFF1A1A1A), RoundedCornerShape(12.dp))
                        .padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Connected",
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFF4CAF50)
                        )
                        igAccountId?.let {
                            Text(
                                text = "Account: $it",
                                fontSize = 12.sp,
                                color = Color.Gray
                            )
                        }
                    }
                    TextButton(
                        onClick = {
                            igLoading = true
                            scope.launch {
                                ComradApp.instance.api.disconnectInstagram()
                                    .onSuccess {
                                        igConnected = false
                                        igAccountId = null
                                    }
                                igLoading = false
                            }
                        },
                        enabled = !igLoading
                    ) {
                        Text("Disconnect", color = Color(0xFFFF5252))
                    }
                }
            } else {
                // Not connected — show connect button
                igError?.let {
                    Text(
                        text = it,
                        color = MaterialTheme.colorScheme.error,
                        fontSize = 14.sp,
                        modifier = Modifier.padding(bottom = 8.dp)
                    )
                }

                Button(
                    onClick = {
                        val appId = ComradApp.instance.instagramAppId
                        if (appId.isBlank()) {
                            igError = "Instagram App ID not configured"
                            return@Button
                        }
                        val redirectUri = "comradwatch://instagram-callback"
                        val authUrl = "https://api.instagram.com/oauth/authorize" +
                            "?client_id=$appId" +
                            "&redirect_uri=$redirectUri" +
                            "&scope=instagram_basic,instagram_content_publish" +
                            "&response_type=code"
                        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(authUrl))
                        context.startActivity(intent)
                    },
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(56.dp),
                    shape = RoundedCornerShape(12.dp),
                    colors = ButtonDefaults.buttonColors(
                        containerColor = Color(0xFFE1306C)
                    ),
                    enabled = !igLoading
                ) {
                    if (igLoading) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(24.dp),
                            color = Color.White
                        )
                    } else {
                        Text(
                            text = "CONNECT INSTAGRAM",
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color.White
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(40.dp))

            // Done button
            Button(
                onClick = onSetupComplete,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(56.dp),
                shape = RoundedCornerShape(12.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary
                )
            ) {
                Text(
                    text = "DONE",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}

/**
 * Handles the Instagram OAuth redirect.
 * Called from MainActivity when the deep link comradwatch://instagram-callback?code=... is received.
 */
suspend fun handleInstagramCallback(code: String): Result<String> {
    val redirectUri = "comradwatch://instagram-callback"
    return ComradApp.instance.api.connectInstagram(code, redirectUri).map { it.username }
}
