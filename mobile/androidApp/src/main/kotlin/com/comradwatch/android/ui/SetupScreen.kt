package com.comradwatch.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
 */
@Composable
fun SetupScreen(onSetupComplete: () -> Unit) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var serverUrl by remember { mutableStateOf("http://10.0.2.2:8080") }
    var isLoading by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }
    var isRegistering by remember { mutableStateOf(true) }

    val scope = rememberCoroutineScope()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0A0A0A))
            .padding(24.dp),
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
            text = "One-time setup",
            fontSize = 16.sp,
            color = Color.Gray,
            textAlign = TextAlign.Center
        )
        Spacer(modifier = Modifier.height(40.dp))

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
                    result.onSuccess { onSetupComplete() }
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
    }
}
