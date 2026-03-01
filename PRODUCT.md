# Comrad Watch

**Your footage survives, even when your phone doesn't.**

---

## The Problem

Every year, journalists, protesters, and human rights observers lose critical video evidence. Phones get seized. Phones get smashed. Phones get confiscated at the worst possible moment — right when the footage matters most.

Existing solutions fail here:
- **Cloud backups** (iCloud, Google Photos) upload *after* recording stops — if the phone is destroyed mid-recording, the footage is gone
- **Live streaming** (Instagram Live, Facebook Live) requires staying in one app with stable internet — unrealistic in chaotic situations
- **Encrypted storage** protects data on-device, but if the device is taken, the footage goes with it

The core issue: **video lives on the phone until it's too late.**

## The Solution

Comrad Watch streams video off the phone the instant recording begins. Every second of footage is saved on a remote server in real-time. If the phone is seized, smashed, or runs out of battery mid-recording — **the server already has the footage.**

One tap. That's it. No menus, no settings to fumble with, no accounts to remember in the moment. Tap the red button, and your footage is being saved somewhere safe before you even start walking.

## How It Works

**For the user:**
1. Open the app. Tap the big red circle.
2. Your phone streams video to a secure server in real-time.
3. When you stop (or your phone is destroyed), the recording is automatically:
   - Saved to your Google Drive as an MP4 file
   - Posted to your Instagram Story (optional)
4. Done. Your footage exists in multiple places, none of which are your phone.

**What makes this different:**
- Video leaves your phone *while you're recording* — not after
- Server-side buffering means footage survives phone destruction
- Automatic backup to Google Drive means you own your footage
- Instagram Story posting means instant public visibility
- No account creation needed to start recording (sign up once, stream forever)

## Who This Is For

- **Journalists** covering protests, conflict zones, or hostile environments
- **Human rights organizations** documenting abuses in the field
- **Activists and protesters** who need their footage to survive police encounters
- **Citizen journalists** recording incidents where phones are routinely confiscated
- **Legal observers** who need tamper-evident, timestamped video evidence

## Why Now

Phone seizure and destruction during documentation is increasing globally. Current tools were built for convenience, not survival. Comrad Watch is built for the moment when everything goes wrong.

## Platform Support

- **Android** — Native app, available now
- **iPhone/iPad** — Web app via Safari (add to home screen for app-like experience)
- **Any modern browser** — Works on desktop and mobile via the web app

## Privacy and Security

- All video streams are encrypted in transit
- OAuth tokens stored with AES-256 encryption at rest
- No third-party analytics or tracking
- Server can be self-hosted by organizations who need full control
- Stream keys are unique per session and never reused

## Technical Architecture (High Level)

```
Phone → RTMP Stream → Server → Google Drive
                           ↘ Instagram Story
```

The phone streams video using RTMP (the same protocol used by Twitch and YouTube Live). The server records every frame as it arrives. When the stream ends — for any reason — the server converts the recording to MP4 and distributes it to the user's connected accounts.

## Status

| Component | Status |
|-----------|--------|
| Backend server (video ingest + API) | Complete |
| Android app | Complete |
| Google Drive auto-upload | Complete |
| Instagram Story auto-post | Complete |
| Web app (iOS + desktop) | Complete |

## Get Involved

Comrad Watch is open source. If your organization needs this tool, or you want to contribute, reach out.

**Built for the people who need their footage to survive.**
