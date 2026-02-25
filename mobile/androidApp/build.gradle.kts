plugins {
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.kotlinAndroid)
    alias(libs.plugins.composeCompiler)
}

android {
    namespace = "com.comradwatch.android"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.comradwatch.android"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    // Shared KMP module
    implementation(project(":shared"))

    // Compose
    val composeBom = platform(libs.compose.bom)
    implementation(composeBom)
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.graphics)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.compose.material3)
    debugImplementation(libs.compose.ui.tooling)

    // AndroidX
    implementation(libs.activity.compose)
    implementation(libs.lifecycle.runtime)
    implementation(libs.navigation.compose)

    // RootEncoder (RTMP streaming — includes camera handling internally)
    implementation(libs.rootencoder)

    // Coroutines
    implementation(libs.kotlinx.coroutines.android)
}
