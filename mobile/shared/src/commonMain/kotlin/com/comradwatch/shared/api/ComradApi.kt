package com.comradwatch.shared.api

import com.comradwatch.shared.model.*
import io.ktor.client.*
import io.ktor.client.call.*
import io.ktor.client.plugins.contentnegotiation.*
import io.ktor.client.request.*
import io.ktor.http.*
import io.ktor.serialization.kotlinx.json.*
import kotlinx.serialization.json.Json

/**
 * API client shared between Android and iOS.
 * Handles all communication with the Go backend.
 */
class ComradApi(private val baseUrl: String) {

    private val client = HttpClient {
        install(ContentNegotiation) {
            json(Json {
                ignoreUnknownKeys = true
                isLenient = true
            })
        }
    }

    private var authToken: String? = null

    fun setToken(token: String) {
        authToken = token
    }

    fun clearToken() {
        authToken = null
    }

    /** Fetch public server config (Instagram App ID, etc.). */
    suspend fun getConfig(): Result<ServerConfigResponse> {
        return try {
            val response = client.get("$baseUrl/api/config")
            if (response.status == HttpStatusCode.OK) {
                Result.success(response.body<ServerConfigResponse>())
            } else {
                Result.failure(Exception("Failed to fetch config"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Register a new account. Returns auth token + user info. */
    suspend fun register(email: String, password: String): Result<AuthResponse> {
        return try {
            val response = client.post("$baseUrl/api/register") {
                contentType(ContentType.Application.Json)
                setBody(RegisterRequest(email, password))
            }
            if (response.status == HttpStatusCode.Created) {
                val auth = response.body<AuthResponse>()
                authToken = auth.token
                Result.success(auth)
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Log in with existing credentials. */
    suspend fun login(email: String, password: String): Result<AuthResponse> {
        return try {
            val response = client.post("$baseUrl/api/login") {
                contentType(ContentType.Application.Json)
                setBody(LoginRequest(email, password))
            }
            if (response.status == HttpStatusCode.OK) {
                val auth = response.body<AuthResponse>()
                authToken = auth.token
                Result.success(auth)
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Start a new streaming session. Returns RTMP URL + stream key. */
    suspend fun startSession(): Result<StartSessionResponse> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.post("$baseUrl/api/sessions/start") {
                header("Authorization", "Bearer $token")
                contentType(ContentType.Application.Json)
            }
            if (response.status == HttpStatusCode.Created) {
                Result.success(response.body<StartSessionResponse>())
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // --- Google Drive (Phase 3) ---

    /** Get the Google OAuth authorization URL. Open this in a browser. */
    suspend fun getGoogleAuthUrl(): Result<String> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.get("$baseUrl/api/google/auth-url") {
                header("Authorization", "Bearer $token")
            }
            if (response.status == HttpStatusCode.OK) {
                val body = response.body<GoogleAuthURLResponse>()
                Result.success(body.url)
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Check if the user has connected Google Drive. */
    suspend fun getGoogleDriveStatus(): Result<Boolean> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.get("$baseUrl/api/google/status") {
                header("Authorization", "Bearer $token")
            }
            if (response.status == HttpStatusCode.OK) {
                val body = response.body<GoogleStatusResponse>()
                Result.success(body.connected)
            } else {
                Result.failure(Exception("Failed to check Drive status"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    // --- Instagram ---

    /** Connect Instagram by exchanging an OAuth authorization code. */
    suspend fun connectInstagram(code: String, redirectUri: String): Result<ConnectInstagramResponse> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.post("$baseUrl/api/instagram/connect") {
                header("Authorization", "Bearer $token")
                contentType(ContentType.Application.Json)
                setBody(ConnectInstagramRequest(code, redirectUri))
            }
            if (response.status == HttpStatusCode.OK) {
                Result.success(response.body<ConnectInstagramResponse>())
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Check if the user has connected Instagram. */
    suspend fun getInstagramStatus(): Result<InstagramStatusResponse> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.get("$baseUrl/api/instagram/status") {
                header("Authorization", "Bearer $token")
            }
            if (response.status == HttpStatusCode.OK) {
                Result.success(response.body<InstagramStatusResponse>())
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** Disconnect Instagram from the user's account. */
    suspend fun disconnectInstagram(): Result<Unit> {
        return try {
            val token = authToken ?: return Result.failure(Exception("Not logged in"))
            val response = client.delete("$baseUrl/api/instagram/disconnect") {
                header("Authorization", "Bearer $token")
            }
            if (response.status == HttpStatusCode.OK) {
                Result.success(Unit)
            } else {
                val error = response.body<ErrorResponse>()
                Result.failure(Exception(error.error))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
}
