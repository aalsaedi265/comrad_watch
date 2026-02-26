package com.comradwatch.shared.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// --- API Request/Response models ---

@Serializable
data class RegisterRequest(
    val email: String,
    val password: String
)

@Serializable
data class LoginRequest(
    val email: String,
    val password: String
)

@Serializable
data class AuthResponse(
    val token: String,
    val user: UserInfo
)

@Serializable
data class UserInfo(
    val id: String,
    val email: String
)

@Serializable
data class StartSessionResponse(
    @SerialName("session_id") val sessionId: String,
    @SerialName("stream_key") val streamKey: String,
    @SerialName("rtmp_url") val rtmpUrl: String
)

@Serializable
data class ErrorResponse(
    val error: String
)

// --- Google Drive (Phase 3) ---

@Serializable
data class GoogleAuthURLResponse(
    val url: String
)

@Serializable
data class GoogleStatusResponse(
    val connected: Boolean
// --- Server config ---

@Serializable
data class ServerConfigResponse(
    @SerialName("instagram_app_id") val instagramAppId: String = ""
)

// --- Instagram ---

@Serializable
data class ConnectInstagramRequest(
    val code: String,
    @SerialName("redirect_uri") val redirectUri: String
)

@Serializable
data class ConnectInstagramResponse(
    val username: String,
    @SerialName("account_id") val accountId: String
)

@Serializable
data class InstagramStatusResponse(
    val connected: Boolean,
    @SerialName("account_id") val accountId: String? = null
)

// --- Local app state ---

data class StreamConfig(
    val serverUrl: String,
    val streamKey: String,
    val sessionId: String
)
