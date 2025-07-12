# WebRTC Issue Reproduction: Pion Peer + Signaling + Browser Client

This repository reproduces a WebRTC connection issue involving [Pion WebRTC](https://github.com/pion/webrtc) when **host candidates are filtered out**. It includes:

- A signaling server (`signaling/main.go`)
- A Go-based Pion WebRTC peer (`peer/main.go`)
- A browser peer (in `chrome.html`)
- WebSocket relay messaging between peers

The goal is to reproduce a scenario where the Go peer fails to establish a connection with the browser peer when host candidates are disabled — which can happen behind NAT or with interface filtering.

## ❌ When Host Candidates Are Filtered

If `FILTER_HOST_CAND=true` and you're testing on `localhost` (i.e., same machine), the connection may **fail silently** due to lack of valid ICE candidates (e.g., no NAT hairpinning or loopback support).

> **Important:** This issue does **not** reproduce properly on a single machine. You must use a **VPS + remote browser** to observe it reliably.

---

## 🧪 Prerequisites

- Go 1.20+
- A publicly accessible VPS (for signaling server & Go peer)
- A browser (Chrome recommended)

---

## ⚙️ Environment Variables (.env)

Create a `.env` file in the root or set environment variables manually.

```dotenv
# Filter out host candidates (true = drop all host candidates)
FILTER_HOST_CAND=false

# Port for the signaling server
SIGNALING_PORT=8081
```

The .env file is loaded automatically using joho/godotenv.

🚀 How to Run (Minimal Working Setup)

1. Run signaling server (on VPS)
   ```bash   
   go run signaling/main.go
   ```
   This starts a WebSocket server on SIGNALING_PORT (default: 8081 or as defined in .env).

2. Run Go-based peer (on same VPS)
   ```bash   
   go run peer/main.go
   ```
   It connects to the signaling server on localhost:$SIGNALING_PORT.

Make sure .env is present in the root with correct values.

3. Open browser peer (on local PC)
   Open chrome.html via file or server, but include ?ws= query param:

For example, open in browser:

file:///path/to/chrome.html?ws=ws://<VPS_IP>:8081/ws
If the ?ws= parameter is missing or invalid, an error will be shown on the page.

✅ Expected Output
On the Go peer, if candidates are allowed (FILTER_HOST_CAND=false), you should see:
✅ DataChannel opened (answerer)

However, with FILTER_HOST_CAND=false, "DataChannel opened" may fail intermittently. Please, run the test several times (5-10) and you should see at least several fails.
