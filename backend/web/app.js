// Comrad Watch PWA — Plain JS, no framework
'use strict';

// ==================== State ====================
var state = {
    token: null,
    serverUrl: null,
    sessionId: null,
    streamKey: null,
    mediaRecorder: null,
    cameraStream: null,
    timerInterval: null,
    elapsedSeconds: 0,
    uploadActive: false,
    startingRecording: false, // guard against double-tap
    driveConnected: null,
    igConnected: null,
    igAccountId: null,
    instagramAppId: null,
    authMode: 'login',
    mimeType: '',
};

// ==================== Init ====================
document.addEventListener('DOMContentLoaded', function() {
    loadPersistedState();
    registerServiceWorker();
    handleInstagramCallback();

    if (state.token) {
        showScreen('main');
        refreshMainScreenStatus();
    } else {
        showScreen('auth');
    }

    attachEventListeners();
});

function loadPersistedState() {
    state.token = localStorage.getItem('comrad_token');
    state.serverUrl = window.location.origin;
}

function registerServiceWorker() {
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/sw.js').catch(function(e) {
            console.warn('SW registration failed:', e);
        });
    }
}

// ==================== Screen Navigation ====================
function showScreen(id) {
    document.querySelectorAll('.screen').forEach(function(s) {
        s.classList.add('hidden');
    });
    document.getElementById('screen-' + id).classList.remove('hidden');
}

// ==================== API Layer ====================
function api(method, path, body, requiresAuth) {
    if (requiresAuth === undefined) requiresAuth = true;
    var url = state.serverUrl + path;
    var headers = { 'Content-Type': 'application/json' };
    if (requiresAuth && state.token) {
        headers['Authorization'] = 'Bearer ' + state.token;
    }
    var opts = { method: method, headers: headers };
    if (body) opts.body = JSON.stringify(body);
    return fetch(url, opts).then(function(res) {
        // Handle expired token — redirect to login
        if (res.status === 401 && requiresAuth) {
            state.token = null;
            localStorage.removeItem('comrad_token');
            showScreen('auth');
            throw new Error('Session expired. Please log in again.');
        }
        if (!res.ok) {
            return res.json().catch(function() {
                return { error: res.statusText };
            }).then(function(err) {
                throw new Error(err.error || res.statusText);
            });
        }
        return res.json();
    });
}

function uploadChunk(sessionId, blob) {
    var url = state.serverUrl + '/api/sessions/' + sessionId + '/chunk';
    return fetch(url, {
        method: 'POST',
        headers: {
            'Authorization': 'Bearer ' + state.token,
            'Content-Type': blob.type || 'application/octet-stream',
        },
        body: blob,
    }).then(function(res) {
        if (res.status === 401) {
            state.token = null;
            localStorage.removeItem('comrad_token');
            stopRecording(true);
            showScreen('auth');
            throw new Error('Session expired');
        }
        if (!res.ok) throw new Error('chunk upload failed: ' + res.status);
        return res.json();
    });
}

function uploadWithRetry(sessionId, blob, attempt) {
    if (!attempt) attempt = 0;
    var maxRetries = 3;
    return uploadChunk(sessionId, blob).catch(function(err) {
        if (attempt < maxRetries && state.uploadActive) {
            var delay = Math.min(1000 * Math.pow(2, attempt), 8000);
            return new Promise(function(resolve, reject) {
                setTimeout(function() {
                    uploadWithRetry(sessionId, blob, attempt + 1)
                        .then(resolve).catch(reject);
                }, delay);
            });
        }
        throw err;
    });
}

// ==================== Auth ====================
function doLogin() {
    var email = document.getElementById('email').value.trim();
    var password = document.getElementById('password').value;

    if (!email || !password) {
        showError('auth-error', 'All fields are required');
        return;
    }

    var btn = document.getElementById('btn-login');
    setButtonLoading(btn, 'LOGGING IN...');

    api('POST', '/api/login', { email: email, password: password }, false)
        .then(function(data) {
            state.token = data.token;
            localStorage.setItem('comrad_token', data.token);
            hideError('auth-error');
            showScreen('main');
            refreshMainScreenStatus();
            showSetupPromptIfNeeded();
        })
        .catch(function(e) {
            showError('auth-error', e.message);
        })
        .finally(function() {
            clearButtonLoading(btn, 'LOG IN');
        });
}

function doRegister() {
    var email = document.getElementById('email').value.trim();
    var password = document.getElementById('password').value;
    var passwordConfirm = document.getElementById('password-confirm').value;

    if (!email || !password || !passwordConfirm) {
        showError('auth-error', 'All fields are required');
        return;
    }

    if (password !== passwordConfirm) {
        showError('auth-error', 'Passwords do not match');
        return;
    }

    var btn = document.getElementById('btn-register');
    setButtonLoading(btn, 'CREATING...');

    api('POST', '/api/register', { email: email, password: password }, false)
        .then(function(data) {
            state.token = data.token;
            localStorage.setItem('comrad_token', data.token);
            hideError('auth-error');
            showScreen('main');
            refreshMainScreenStatus();
            showSetupPromptIfNeeded();
        })
        .catch(function(e) {
            showError('auth-error', e.message);
        })
        .finally(function() {
            clearButtonLoading(btn, 'CREATE ACCOUNT');
        });
}

function toggleAuthMode() {
    var btn = document.getElementById('btn-toggle-auth');
    var loginBtn = document.getElementById('btn-login');
    var registerBtn = document.getElementById('btn-register');
    var confirmGroup = document.getElementById('password-confirm-group');

    if (state.authMode === 'login') {
        state.authMode = 'register';
        btn.textContent = 'Already have an account? Log in';
        loginBtn.classList.add('hidden');
        registerBtn.classList.remove('hidden');
        confirmGroup.classList.remove('hidden');
    } else {
        state.authMode = 'login';
        btn.textContent = 'Need an account? Register';
        loginBtn.classList.remove('hidden');
        registerBtn.classList.add('hidden');
        confirmGroup.classList.add('hidden');
        document.getElementById('password-confirm').value = '';
    }
}

function logout() {
    // Stop active recording before logging out
    if (state.mediaRecorder && state.mediaRecorder.state !== 'inactive') {
        stopRecording(true);
    }
    // Clean up camera if still open
    if (state.cameraStream) {
        state.cameraStream.getTracks().forEach(function(t) { t.stop(); });
        state.cameraStream = null;
    }
    state.token = null;
    state.sessionId = null;
    state.streamKey = null;
    state.driveConnected = null;
    state.igConnected = null;
    localStorage.removeItem('comrad_token');
    showScreen('auth');
}

// ==================== Main Screen ====================
function refreshMainScreenStatus() {
    api('GET', '/api/google/status').then(function(data) {
        state.driveConnected = data.connected;
        updateDriveChip();
    }).catch(function() {
        state.driveConnected = null;
        updateDriveChip();
    });
}

function updateDriveChip() {
    var chip = document.getElementById('drive-chip');
    if (state.driveConnected === null) {
        chip.classList.add('hidden');
        return;
    }
    chip.classList.remove('hidden');
    if (state.driveConnected) {
        chip.textContent = 'Drive \u2713';
        chip.className = 'status-chip bottom-left connected';
        chip.onclick = null;
    } else {
        chip.textContent = 'Connect Drive';
        chip.className = 'status-chip bottom-left disconnected';
        chip.onclick = connectDrive;
    }
}

// ==================== Recording ====================
function startRecording() {
    // Guard: prevent double-tap from creating duplicate sessions
    if (state.startingRecording || state.uploadActive || state.sessionId) return;

    if (!window.MediaRecorder) {
        alert('Recording is not supported on this browser. Please update to iOS 14.5+ or use a modern browser.');
        return;
    }

    state.startingRecording = true;

    // Request camera
    navigator.mediaDevices.getUserMedia({
        video: { facingMode: { ideal: 'environment' }, width: { ideal: 1280 }, height: { ideal: 720 } },
        audio: true,
    }).then(function(stream) {
        state.cameraStream = stream;

        // Start session on server
        return api('POST', '/api/sessions/start', {}).then(function(session) {
            state.sessionId = session.session_id;
            state.streamKey = session.stream_key;
            state.elapsedSeconds = 0;
            state.uploadActive = true;
            state.startingRecording = false;

            // Ensure stop menu is hidden for fresh recording
            document.getElementById('stop-menu').classList.add('hidden');

            // Show recording screen
            showScreen('recording');
            var video = document.getElementById('camera-preview');
            video.srcObject = stream;
            video.play().catch(function() {}); // explicit play for Safari

            // Pick MIME type
            state.mimeType = pickMimeType();

            // Set up MediaRecorder
            var options = {};
            if (state.mimeType) options.mimeType = state.mimeType;
            state.mediaRecorder = new MediaRecorder(stream, options);

            state.mediaRecorder.ondataavailable = function(e) {
                if (!state.uploadActive || e.data.size === 0) return;
                uploadWithRetry(state.sessionId, e.data).then(function() {
                    setStatus('LIVE', 'live');
                }).catch(function() {
                    setStatus('UPLOAD FAILED', 'warn');
                });
            };

            state.mediaRecorder.onerror = function(e) {
                console.error('MediaRecorder error:', e);
                stopRecording(false);
            };

            // Record in 2-second chunks
            state.mediaRecorder.start(2000);
            setStatus('CONNECTING', 'connecting');

            // Timer
            document.getElementById('timer').textContent = '00:00';
            state.timerInterval = setInterval(function() {
                state.elapsedSeconds++;
                document.getElementById('timer').textContent = formatTime(state.elapsedSeconds);
            }, 1000);
        });
    }).catch(function(e) {
        state.startingRecording = false;
        // Clean up camera if it was opened but session-start failed
        if (state.cameraStream) {
            state.cameraStream.getTracks().forEach(function(t) { t.stop(); });
            state.cameraStream = null;
        }
        alert('Camera access required. Please allow camera and microphone permissions.\n\n' + e.message);
    });
}

function pickMimeType() {
    var types = [
        'video/mp4;codecs=h264,aac',
        'video/mp4',
        'video/webm;codecs=vp9,opus',
        'video/webm;codecs=vp8,opus',
        'video/webm',
    ];
    for (var i = 0; i < types.length; i++) {
        if (MediaRecorder.isTypeSupported(types[i])) return types[i];
    }
    return '';
}

function stopRecording(discard) {
    state.uploadActive = false;
    clearInterval(state.timerInterval);

    // Hide stop menu
    document.getElementById('stop-menu').classList.add('hidden');

    var afterStop = function() {
        // Stop camera
        if (state.cameraStream) {
            state.cameraStream.getTracks().forEach(function(t) { t.stop(); });
            state.cameraStream = null;
        }

        if (!discard && state.sessionId) {
            // Signal server to finalize
            api('POST', '/api/sessions/' + state.sessionId + '/end', {
                mime_type: state.mimeType
            }).then(function() {
                showSaveConfirmation();
            }).catch(function(e) {
                console.error('end session failed:', e);
            });
        }

        state.sessionId = null;
        state.streamKey = null;
        state.mediaRecorder = null;

        showScreen('main');
        refreshMainScreenStatus();
    };

    if (state.mediaRecorder && state.mediaRecorder.state !== 'inactive') {
        state.mediaRecorder.onstop = afterStop;
        state.mediaRecorder.stop();
    } else {
        afterStop();
    }
}

// ==================== Settings ====================
function loadSettingsScreen() {
    document.getElementById('settings-server-display').textContent = window.location.origin;

    // Drive status
    var driveEl = document.getElementById('drive-status-settings');
    var driveBtn = document.getElementById('btn-connect-drive');
    driveEl.textContent = 'Checking...';
    driveEl.className = 'status-line off';
    driveBtn.classList.add('hidden');

    api('GET', '/api/google/status').then(function(data) {
        state.driveConnected = data.connected;
        if (data.connected) {
            driveEl.textContent = 'Connected \u2713';
            driveEl.className = 'status-line ok';
        } else {
            driveEl.textContent = 'Not connected';
            driveEl.className = 'status-line off';
            driveBtn.classList.remove('hidden');
        }
    }).catch(function() {
        driveEl.textContent = 'Unable to check';
        driveEl.className = 'status-line off';
    });

    // Instagram status
    var igEl = document.getElementById('ig-status-settings');
    var igBtn = document.getElementById('btn-connect-ig');
    var igDisBtn = document.getElementById('btn-disconnect-ig');
    igEl.textContent = 'Checking...';
    igEl.className = 'status-line off';
    igBtn.classList.add('hidden');
    igDisBtn.classList.add('hidden');

    api('GET', '/api/instagram/status').then(function(data) {
        state.igConnected = data.connected;
        state.igAccountId = data.account_id;
        if (data.connected) {
            igEl.textContent = 'Connected \u2713' + (data.account_id ? ' (' + data.account_id + ')' : '');
            igEl.className = 'status-line ok';
            igDisBtn.classList.remove('hidden');
        } else {
            igEl.textContent = 'Not connected';
            igEl.className = 'status-line off';
            igBtn.classList.remove('hidden');
        }
    }).catch(function() {
        igEl.textContent = 'Unable to check';
        igEl.className = 'status-line off';
    });

    // Load Instagram App ID for OAuth
    api('GET', '/api/config', null, false).then(function(cfg) {
        state.instagramAppId = cfg.instagram_app_id;
    }).catch(function() {});
}

function connectDrive() {
    api('GET', '/api/google/auth-url').then(function(data) {
        window.location.href = data.url;
    }).catch(function(e) {
        alert('Failed to get Google auth URL: ' + e.message);
    });
}

function connectInstagram() {
    if (!state.instagramAppId) {
        alert('Instagram is not configured on this server.');
        return;
    }
    var redirectUri = window.location.origin + '/?ig_callback=1';
    var authUrl = 'https://api.instagram.com/oauth/authorize' +
        '?client_id=' + state.instagramAppId +
        '&redirect_uri=' + encodeURIComponent(redirectUri) +
        '&scope=instagram_basic,instagram_content_publish' +
        '&response_type=code';
    window.location.href = authUrl;
}

function disconnectInstagram() {
    api('DELETE', '/api/instagram/disconnect').then(function() {
        loadSettingsScreen();
    }).catch(function(e) {
        alert('Failed to disconnect: ' + e.message);
    });
}

function handleInstagramCallback() {
    var params = new URLSearchParams(window.location.search);
    if (params.get('ig_callback') && params.get('code')) {
        var code = params.get('code');
        window.history.replaceState({}, '', '/');
        if (state.token && state.serverUrl) {
            api('POST', '/api/instagram/connect', {
                code: code,
                redirect_uri: window.location.origin + '/?ig_callback=1',
            }).then(function() {
                showScreen('settings');
                loadSettingsScreen();
            }).catch(function(e) {
                alert('Instagram connect failed: ' + e.message);
            });
        }
    }
}

// ==================== Setup Prompt ====================
function showSetupPromptIfNeeded() {
    // Only show once per browser — after that the user knows
    if (localStorage.getItem('comrad_setup_seen')) return;
    localStorage.setItem('comrad_setup_seen', '1');
    document.getElementById('modal-setup').classList.remove('hidden');
}

function dismissSetupPrompt() {
    document.getElementById('modal-setup').classList.add('hidden');
}

// ==================== Helpers ====================
function formatTime(seconds) {
    var m = Math.floor(seconds / 60);
    var s = seconds % 60;
    return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
}

function setButtonLoading(btn, text) {
    btn.disabled = true;
    btn.textContent = text;
}

function clearButtonLoading(btn, text) {
    btn.disabled = false;
    btn.textContent = text;
}

function setStatus(text, type) {
    document.getElementById('status-text').textContent = text;
    document.getElementById('status-dot').className = 'status-dot ' + type;
}

function showError(id, msg) {
    var el = document.getElementById(id);
    el.textContent = msg;
    el.classList.remove('hidden');
}

function hideError(id) {
    document.getElementById(id).classList.add('hidden');
}

function showSaveConfirmation() {
    var toast = document.createElement('div');
    toast.className = 'save-toast';
    toast.textContent = 'FOOTAGE SAVED \u2713';
    document.body.appendChild(toast);
    setTimeout(function() {
        toast.classList.add('fade-out');
        setTimeout(function() {
            if (toast.parentNode) toast.parentNode.removeChild(toast);
        }, 500);
    }, 2500);
}

// ==================== Event Listeners ====================
function attachEventListeners() {
    // Auth
    document.getElementById('btn-login').onclick = doLogin;
    document.getElementById('btn-register').onclick = doRegister;
    document.getElementById('btn-toggle-auth').onclick = toggleAuthMode;

    // Main
    document.getElementById('btn-record').onclick = startRecording;
    document.getElementById('btn-settings').onclick = function() {
        showScreen('settings');
        loadSettingsScreen();
    };

    // Recording
    document.getElementById('btn-show-menu').onclick = function() {
        document.getElementById('stop-menu').classList.remove('hidden');
    };
    document.getElementById('btn-cancel-menu').onclick = function() {
        document.getElementById('stop-menu').classList.add('hidden');
    };
    document.getElementById('btn-stop-save').onclick = function() { stopRecording(false); };
    document.getElementById('btn-stop-discard').onclick = function() { stopRecording(true); };

    // Settings
    document.getElementById('btn-back').onclick = function() {
        showScreen('main');
        refreshMainScreenStatus();
    };
    document.getElementById('btn-connect-drive').onclick = connectDrive;
    document.getElementById('btn-connect-ig').onclick = connectInstagram;
    document.getElementById('btn-disconnect-ig').onclick = disconnectInstagram;
    document.getElementById('btn-logout').onclick = logout;

    // Setup modal
    document.getElementById('btn-modal-settings').onclick = function() {
        dismissSetupPrompt();
        showScreen('settings');
        loadSettingsScreen();
    };
    document.getElementById('btn-modal-dismiss').onclick = dismissSetupPrompt;

    // Enter key in auth fields
    ['email', 'password', 'password-confirm'].forEach(function(id) {
        document.getElementById(id).addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                state.authMode === 'login' ? doLogin() : doRegister();
            }
        });
    });
}
