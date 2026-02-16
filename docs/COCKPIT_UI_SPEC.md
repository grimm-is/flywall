# **Router Cockpit UI**

Version: 1.10.0 (Host & Container Mode)
Target User: Power User / Backend Developer
Core Philosophy: High Signal, Low Noise. Function is Aesthetic.

## **0. Data Architecture**

**WebSocket-First:** All live data is pushed via WebSocket topics. Client polling is only a fallback for degraded connections.

### **WebSocket Topics**

| Topic | Data | Update Frequency |
| :---- | :---- | :---- |
| config | Full configuration snapshot | On change |
| leases | Active DHCP leases | On change |
| stats:rules | NFT rule counters (PPS, BPS) | ~2s |
| stats:interfaces | Interface bandwidth, packets | ~2s |
| flows | Active connection table | ~2s |
| health | System health (CPU, mem, temps) | ~5s |
| logs | Log entries (filtered by view) | Real-time stream |
| learning:pending | Pending approval decisions | Real-time |
| learning:fingerprints | Device fingerprints (DHCP, mDNS, JA3/JA3S) | On change |
| learning:anomalies | Detected behavioral anomalies | Real-time |
| runtime:containers | Active local containers/VMs (Host Mode) | On change |

### **Client Behavior**

1. **On connect:** Subscribe to relevant topics for current view
2. **On navigate:** Unsubscribe from irrelevant topics, subscribe to new ones
3. **On disconnect:** Reconnect with exponential backoff, fallback to REST polling if WS unavailable

## **1. Core Principles**

"Don't design for the database schema; design for the mental model."

1. **Entity-Centric:** Users configure a **Zone** which *has* an IP and *has* a DHCP scope—not separate config pages.
2. **Obvious Connections:** Relationships between objects (Zones, Rules, NAT) must be visually explicit.
3. **Locality of Behavior:** If two things interact (Rule + NAT redirect), they live on the same screen.
4. **The "Glass Chassis":** Observability is not a separate app. Live data is everywhere.
5. **Context Agnostic:** Whether managing a physical router or a container host, the interface adapts to the underlying reality (MACs vs PIDs).

## **2. Visual Language**

### **The Vibe: "Modern Terminal"**

* **Aesthetic:** Cyberpunk Dashboard / Fighter Jet HUD
* **Density:** High. Whitespace is for grouping, not for "airiness."

### **Color Palette ("Slate & Semantic")**

#### **Mode A: Deep Slate (Dark) - *Default***

| Role | Hex | Usage |
| :---- | :---- | :---- |
| Canvas | #0f172a | Main background (Deep Slate) |
| Card | #1e293b | Component background |
| Input | #334155 | Search bars, form fields |
| Text Main | #f8fafc | Primary content, headers |
| Text Muted | #94a3b8 | Meta-data, labels |

#### **Mode B: Ice & Ceramic (Light)**

| Role | Hex | Usage |
| :---- | :---- | :---- |
| Canvas | #f1f5f9 | Main background |
| Card | #ffffff | Component (with shadow) |
| Input | #e2e8f0 | Form fields |
| Text Main | #0f172a | Primary content |
| Text Muted | #64748b | Labels |

#### **Semantic Layer (Both Themes)**

| Role | Hex | Usage |
| :---- | :---- | :---- |
| Success | #10b981 | Active, Up (Emerald) |
| Warning | #f59e0b | Degradation (Amber) |
| Critical | #ef4444 | Blocked, Down (Red) |
| Info | #3b82f6 | Selected (Electric Blue) |
| **Virtual** | #a855f7 | **Containers / VMs / Bridges (Purple)** |

### **Typography**

* **UI/Labels:** Inter or Roboto (Clean Sans-Serif)
* **Data/Logs:** JetBrains Mono, Fira Code, or Berkeley Mono
* **Rule:** If it is dynamic data (IP, MAC, Throughput, ID), it is **always** Monospace.

## **3. Interaction Model**

### **Terminology Shift**

| Traditional | Cockpit | Why? |
| :---- | :---- | :---- |
| Interfaces | **Zones** | "Interface" implies hardware. "Zone" implies security boundary. |
| Firewall | **Policy** | "Firewall" is mechanism. "Policy" is intent (Rules + NAT + Shaping). |
| Services | **Capabilities** | DHCP/DNS are capabilities of a Zone, not standalone daemons. |
| System | **Hardware** | Be specific. CPU temps and disk space live here. |

### **3.1 Deployment Contexts**

The UI adapts based on the detected runtime mode:

1. **Router Mode (Default):**
   * Focus: Physical Ports, WAN/LAN, DHCP Leases.
   * Identity: MAC Addresses, Hostnames.
2. **Host/Appliance Mode:**
   * Focus: Virtual Bridges (br0, docker0), Containers, VMs.
   * Identity: Container Names, Process IDs (PIDs), VM Tags.
   * *Visual Cue:* "Virtual" entities use the **Purple** semantic color.

### **3.2 Safety Protocols (Commit-Confirm)**

**Staged State Indicator:**
* Amber dot in header when pending changes exist
* "Changes pending" badge with count

**Apply Flow:**
```
┌─────────────────────────────────────────────────────────────┐
│  ⚠️  PENDING CHANGES (3)        [Discard]  [Apply & Test]   │
└─────────────────────────────────────────────────────────────┘
        ↓ (clicks Apply)
┌─────────────────────────────────────────────────────────────┐
│  🟠  CONFIG WILL REVERT IN 45s...      [Revert Now] [Confirm]│
│  ████████████░░░░░░░░░░░░░░░░░░  (progress bar)              │
└─────────────────────────────────────────────────────────────┘
```

## **4. Navigation Structure (The "Rail")**

5 top-level contexts in a slim left Sidebar. "Rail & Grid" pattern.

| Icon | Name | Purpose |
| :---- | :---- | :---- |
| 🕸️ | **Topology** | Zone Dashboard, **Containers**, Device Inventory |
| 🛡️ | **Policy** | Rules, NAT, Routing, Shaping, Objects |
| 🔭 | **Observatory** | Live traffic, Logs, Route Table |
| 🚇 | **Tunnels** | WireGuard, OpenVPN, IPsec |
| ⚙️ | **System** | Hardware, Uplinks, Users, HCL Editor |

## **5. Screen Specifications**

### **A. The System Grid**

Goal: High-density control panel for non-traffic configuration.

**Host Mode Adaptation:** If running on a hypervisor/container host, "Infrastructure" shows runtime linkage.

### **B. The Zone Dashboard (Topology)**

Goal: "Single Pane of Glass" for network topology (Physical + Virtual).

**Hero Visualization (Host Mode):**
* Shows **Host** → **Bridge** → **Container/VM**.
* Virtual entities use a **dashed border** and **purple accent**.

### **C. The Policy Engine (Unified Flow)**

Goal: Unified intent definition.

**Host Mode Selectors:**
* Source/Dest can be: **IP Address**, **Zone**, **MAC**, or **Container Name**.
* The UI auto-resolves "redis-db" to its current IP(s) via the Runtime Linkage.

### **D. The Pulse (Live Impact Visualization)**

**Backend (Server-Side Collection):**
* **Go Worker:** Queries nftables counters via github.com/google/nftables.
* **Push:** Broadcasts to frontend via WebSocket topic `stats:rules`.

**Frontend (No Polling):**
* **Heatmap Dimming:** Zero-traffic rules (5min) fade to 50% opacity.
* **Action Flashes:** Block/Accept counters trigger visual pulses.

### **E. Observatory (Glass Chassis)**

Goal: Live iftop + DVR Time Travel + Audit.

**Host Mode Enhancement:**
* Source/Destination columns display **Container Names** or **Process Names** instead of just IPs.
* *Implementation:* Backend correlates conntrack entries with Docker/Procfs metadata.

## **6. Interaction Patterns**

### **The "Crowdsourced Correction" Loop**

1. User sees device labeled "Generic Android" in Topology.
2. User clicks "Edit Identity" and selects "Nvidia Shield".
3. User toggles "Share this signature with Community?".
4. Backend strips IP/Hostname, bundles OUI, DHCP Options, TTL, Window Size.
5. Uploads to `api.flywall.com/v1/fingerprint`.

### **The "Command Palette" (Cmd+K)**

Must be wired to **everything**:
* "Edit NTP" → Opens System > Time & NTP
* "Block 192.168.1.5" → Opens Policy with pre-filled block rule
* "Show Containers" → Opens Topology with Docker zone expanded
* "Capture WAN" → Starts tcpdump on WAN interface

## **7. Implementation Roadmap**

### **Phase 1: Visual Shell** ✅
* 5-rail Svelte layout
* Zone cards with Bento grid
* SearchPalette for Cmd+K

### **Phase 2: Data Rewiring** ✅
* Zone Aggregation: Merge config + runtime
* Policy Swimlanes: Group by source zone
* Flows store with polling

### **Phase 3: New Backend Capabilities** ⚠️ (Current)
* Stats Aggregator: nft list counters → WS `stats:rules`
* Runtime Linkage: Docker/Libvirt socket listeners
* Simulation: POST /api/debug/simulate-packet
* Capture: POST /api/debug/capture

## **8. Technical Architecture & API Mapping**

### **Existing Capability Map**

| Cockpit Feature | Backend Endpoint | Status |
| :---- | :---- | :---- |
| Zone Dashboard | /api/config/{zones,interfaces} | ✅ Ready |
| DHCP | /api/config/dhcp | ✅ Ready |
| Container Inventory | /api/runtime/containers | ⚠️ New |
| Rules & NAT | /api/config/{policies,nat} | ✅ Ready |
| Routing (Static/PBR) | /api/config/{routes,policy_routes} | ✅ Ready |
| Uplinks/Multi-WAN | /api/uplinks/* | ✅ Ready |
| Runtime Linkage | /api/system/runtimes | ⚠️ New |
| Audit Logs | /api/audit | ✅ Ready |
| Route Table | /api/system/routes | ✅ Ready |

### **Required Development (The Gap)**

| Feature | Endpoint | Backend |
| :---- | :---- | :---- |
| Container Link | WS: runtime:containers | Docker Socket Listener |
| The Pulse | WS: stats:rules | nftables counter poll |
| Packet Simulator | POST /api/debug/simulate-packet | nft expression eval |
| One-Click Capture | POST /api/debug/capture | gopacket + pcapgo |

## **9. Technical Details**

### **The Pulse Scalability**

1. Backend pushes **Top 50 Active Rules** by traffic.
2. Frontend can request specific rules via viewport context.

### **Mobile/Responsive Strategy**

| Element | Desktop | Mobile (< 768px) |
| :---- | :---- | :---- |
| Navigation | 5-rail left sidebar | Bottom nav bar |
| Zone Card | 3-column bento | Stacked vertical |
| Hero Topology | Full D3 graph | Hidden by default |
| Tables | Horizontal scroll | Card list view |

## **10. The Sentinel (Local AI Stack)**

**Philosophy:** Train heavy (cloud/dev machine), run light (router). No Python on the appliance.

### **Technology: ONNX Runtime**

| Layer | Tool | Location |
| :---- | :---- | :---- |
| Training | Python + Scikit-Learn/LightGBM | Dev machine |
| Export | `skl2onnx` / `torch.onnx` | `.onnx` file (~50KB-500KB) |
| Inference | `github.com/yalue/onnxruntime_go` | Router (Go binary) |

### **Sentinel Models**

| Model | Algorithm | Size | Input | Output | Cockpit Integration |
| :---- | :---- | :---- | :---- | :---- | :---- |
| **Device Classifier** | Random Forest | ~200KB | DHCP opts, JA3, OUI, TTL | Device Category | Topology: auto-icon (Xbox, Nest) |
| **Anomaly Detector** | Autoencoder NN | ~500KB | Flow stats | Reconstruction error | Observatory: "Abnormal behavior" alert |
| **Traffic Classifier** | Decision Tree | ~50KB | Packet sizes, IAT | App Type (Gaming, VoIP) | Policy > QoS: auto-prioritize |

### **Inference Pipeline**

```
┌──────────────────────────────────────────────────────────────────────┐
│  PACKET ARRIVES                                                      │
│      ↓                                                               │
│  [Feature Extract] → IAT, Size, Protocol, JA3 Hash                   │
│      ↓                                                               │
│  [Tensorize] → []float32{1500.0, 6.0, 7.0, ...}                      │
│      ↓                                                               │
│  [ONNX Runtime] → session.Run(inputTensor)                           │
│      ↓                                                               │
│  [Result] → {category: "Security Camera", confidence: 0.85}          │
│      ↓                                                               │
│  [WebSocket Push] → learning:fingerprints                            │
└──────────────────────────────────────────────────────────────────────┘
```

### **WebSocket Topics (Sentinel)**

| Topic | Data | Update |
| :---- | :---- | :---- |
| `sentinel:classification` | Device category + confidence | On new device |
| `sentinel:anomaly` | Flow ID + anomaly score + baseline delta | Real-time |
| `sentinel:traffic` | Flow ID + app type (Gaming/VoIP/Streaming) | ~2s |

### **Cockpit UI Integration**

1. **Topology > Device Card:**
   - Shows ML-classified device type with confidence badge
   - "AI: 85% Security Camera" with option to correct
   - Corrections feed crowdsourced training data

2. **Observatory > Anomaly Alerts:**
   - Red flash when anomaly score > threshold
   - "This thermostat is behaving unlike any thermostat" dialog
   - One-click: Isolate / Approve / Ignore

3. **Policy > QoS:**
   - "Auto-Prioritize by Traffic Type" toggle
   - Shows detected app types with throughput
   - Gaming/VoIP auto-boosted, torrents auto-limited

### **Model Lifecycle**

1. **Ship:** Models bundled with firmware (`/usr/share/flywall/models/`)
2. **Update:** Optional OTA model updates (smaller than full firmware)
3. **Feedback:** User corrections → anonymized → cloud training
