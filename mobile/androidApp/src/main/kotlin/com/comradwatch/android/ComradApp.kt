package com.comradwatch.android

import android.app.Application
import com.comradwatch.shared.api.ComradApi

class ComradApp : Application() {

    // Single API instance shared across the app.
    // Server URL is set during first-time setup.
    lateinit var api: ComradApi
        private set

    override fun onCreate() {
        super.onCreate()
        instance = this
        // Default to localhost for dev; user configures real server in settings
        api = ComradApi("http://10.0.2.2:8080") // Android emulator → host machine
    }

    fun updateServerUrl(url: String) {
        api = ComradApi(url)
    }

    companion object {
        lateinit var instance: ComradApp
            private set
    }
}
