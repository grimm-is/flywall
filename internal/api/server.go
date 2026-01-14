package api

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"grimm.is/flywall/internal/clock"

	"grimm.is/flywall/internal/api/storage"
	"grimm.is/flywall/internal/auth"
	"grimm.is/flywall/internal/brand"
	"grimm.is/flywall/internal/config"
	"grimm.is/flywall/internal/ctlplane"
	"grimm.is/flywall/internal/i18n"
	"grimm.is/flywall/internal/learning"
	"grimm.is/flywall/internal/logging"
	"grimm.is/flywall/internal/metrics"
	"grimm.is/flywall/internal/ratelimit"
	"grimm.is/flywall/internal/sentinel"
	"grimm.is/flywall/internal/state"
	"grimm.is/flywall/internal/stats"
	"grimm.is/flywall/internal/tls"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"grimm.is/flywall/internal/runtime"
)

func init() {
	// Ensure MIME types are registered, as minimal environments might lack /etc/mime.types
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".html", "text/html")
	mime.AddExtensionType(".json", "application/json")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".png", "image/png")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".wasm", "application/wasm")
}

// ServerConfig holds HTTP server security configuration.
// Mitigation: OWASP A05:2021-Security Misconfiguration
type ServerConfig struct {
	ReadHeaderTimeout time.Duration // Slowloris prevention
	ReadTimeout       time.Duration // Body read limit
	WriteTimeout      time.Duration // Response timeout
	IdleTimeout       time.Duration // Keep-alive timeout
	MaxHeaderBytes    int           // Header size limit
	MaxBodyBytes      int64         // Request body size limit
}

// DefaultServerConfig returns secure default server configuration.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadHeaderTimeout: 10 * time.Second, // Slowloris prevention
		ReadTimeout:       15 * time.Second, // Body read limit
		WriteTimeout:      30 * time.Second, // Response timeout
		IdleTimeout:       60 * time.Second, // Keep-alive timeout
		MaxHeaderBytes:    1 << 16,          // 64KB header limit
		MaxBodyBytes:      10 << 20,         // 10MB body limit
	}
}

// Server handles API requests.
type Server struct {
	Config          *config.Config
	Assets          fs.FS
	client          ctlplane.ControlPlaneClient // RPC client for control plane communication
	authStore       auth.AuthStore
	authMw          *auth.Middleware
	apiKeyManager   *APIKeyManager // Use local type alias/import
	logger          *logging.Logger
	collector       *metrics.Collector
	startTime       time.Time
	learning        *learning.Service
	sentinel        *sentinel.Service // Device Fingerprinting
	stateStore      state.Store
	runtime         *runtime.DockerClient
	configMu        sync.RWMutex       // Mutex to protect Config access
	csrfManager     *CSRFManager       // CSRF token manager
	rateLimiter     *ratelimit.Limiter // Rate limiter for auth endpoints
	security        *SecurityManager   // Security manager for IP blocking
	wsManager       *WSManager         // Websocket manager
	healthy         atomic.Bool        // Cached health status
	adminCreationMu sync.Mutex         // Mutex to prevent race conditions in admin creation

	// ClearPath Policy Editor support
	statsCollector *stats.Collector // Rule stats for sparklines
	deviceLookup   DeviceLookup     // Device name resolution for UI pills

	mux *http.ServeMux
}

// ServerOptions holds dependencies for the API server
type ServerOptions struct {
	Config          *config.Config
	Assets          fs.FS
	Client          ctlplane.ControlPlaneClient
	AuthStore       auth.AuthStore
	APIKeyManager   *APIKeyManager
	Logger          *logging.Logger
	StateStore      state.Store       // Optional: For standalone mode
	LearningService *learning.Service // Optional: For standalone mode
}

// NewServer creates a new API server with the provided options
func NewServer(opts ServerOptions) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = logging.New(logging.DefaultConfig())
	}

	collector := metrics.NewCollector(logger, 30*time.Second)

	// Initialize security components
	csrfManager := NewCSRFManager()
	rateLimiter := ratelimit.NewLimiter()
	rateLimiter.StartCleanup(10*time.Minute, 1*time.Hour)

	// Create common server structure
	logger.Info("[DEBUG] NewServer initialized (Binary Updated Verify)")
	s := &Server{
		Config:        opts.Config,
		Assets:        opts.Assets,
		logger:        logger,
		collector:     collector,
		startTime:     clock.Now(),
		csrfManager:   csrfManager,
		rateLimiter:   rateLimiter,
		client:        opts.Client,
		authStore:     opts.AuthStore,
		apiKeyManager: opts.APIKeyManager,
		stateStore:    opts.StateStore,
		learning:      opts.LearningService,
		sentinel:      sentinel.New(),
	}

	// Setup auth store: use DevStore if no auth configured
	if opts.AuthStore != nil {
		s.authStore = opts.AuthStore
		s.authMw = auth.NewMiddleware(opts.AuthStore)
	} else if opts.Config != nil && opts.Config.API != nil && !opts.Config.API.RequireAuth {
		// Explicitly allowed no-auth mode
		s.logger.Info("Authentication disabled by configuration")
	} else {
		// Fallback or error?
		// To be safe and compliant with the "Hardening" goal, we should NOT strictly default to DevStore invisible.
		// However, existing tests might rely on this.
		// If we strictly follow the plan: "Remove default fallback to auth.NewDevStore()".
		// If we just remove it, s.authStore remains nil.
		// We need to ensure 'require' doesn't panic.
	}

	// Start WebSocket manager
	s.wsManager = NewWSManager(s.client, s.checkPendingStatus)
	if s.runtime != nil {
		s.wsManager.SetRuntimeService(s.runtime)
	}

	// Initialize Security Manager for fail2ban-style blocking
	// Note: IPSetService integration via RPC will be added when client is available
	s.security = NewSecurityManager(opts.Client, logger)

	// Start background health check
	go s.runHealthCheck()

	s.initRoutes()
	return s, nil
}

// runHealthCheck periodically updates the health status
func (s *Server) runHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	check := func() {
		// Lightweight library-based check
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := checkNFTables(ctx); err != nil {
			s.healthy.Store(false)
		} else {
			s.healthy.Store(true)
		}
	}

	// Initial check
	check()

	for range ticker.C {
		check()
	}
}

// initRoutes initializes the HTTP router
func (s *Server) initRoutes() {
	mux := http.NewServeMux()
	s.mux = mux

	// Public endpoints (no auth required)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/setup/create-admin", s.handleCreateAdmin)
	mux.HandleFunc("GET /api/status", s.handleStatus) // Health status - public for monitoring

	// Batch API
	// Rate limiting for Batch API is applied inside the handler to account for batch size
	mux.HandleFunc("POST /api/batch", s.handleBatch)

	// Websockets
	mux.HandleFunc("GET /api/ws/status", s.handleStatusWS)

	// Branding (public)
	mux.HandleFunc("GET /api/brand", s.handleBrand)

	// OpenAPI & Docs
	mux.HandleFunc("GET /api/openapi.yaml", s.handleOpenAPI)
	mux.HandleFunc("GET /api/docs", s.handleSwaggerUI)

	// UI Schema endpoints (public - needed before auth)
	mux.HandleFunc("GET /api/ui/menu", s.handleUIMenu)
	mux.HandleFunc("GET /api/ui/pages", s.handleUIPages)
	mux.HandleFunc("GET /api/ui/page/", s.handleUIPage)

	// Health check endpoints (public - for monitoring)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReadiness)

	// Protected endpoints - using Unified Auth (User Session or API Key)
	// DevStore is used when no auth configured, providing full access

	// General Config
	mux.Handle("GET /api/config", s.require(storage.PermReadConfig, s.requireControlPlane(s.handleConfig)))
	mux.Handle("POST /api/config/apply", s.require(storage.PermWriteConfig, s.requireControlPlane(s.handleApplyConfig)))
	mux.Handle("POST /api/config/safe-apply", s.require(storage.PermWriteConfig, s.requireControlPlane(s.handleSafeApply)))
	mux.Handle("POST /api/config/confirm", s.require(storage.PermWriteConfig, s.requireControlPlane(s.handleConfirmApply)))
	mux.Handle("GET /api/config/pending", s.require(storage.PermReadConfig, s.requireControlPlane(s.handlePendingApply)))
	mux.Handle("POST /api/config/ip-forwarding", s.require(storage.PermWriteConfig, s.requireControlPlane(s.handleSetIPForwarding)))
	mux.Handle("POST /api/config/settings", s.require(storage.PermWriteConfig, s.requireControlPlane(s.handleSystemSettings)))

	// Upgrade
	mux.Handle("POST /api/system/upgrade", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleSystemUpgrade)))

	// Scanner
	scannerHandlers := NewScannerHandlers(s, s.client)
	scannerHandlers.RegisterRoutes(mux)

	// Status & Metrics (status is public, registered above)
	mux.Handle("GET /api/traffic", s.require(storage.PermReadMetrics, http.HandlerFunc(s.handleTraffic)))

	// User Management
	mux.Handle("GET /api/users", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleGetUsers)))
	mux.Handle("POST /api/users", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleCreateUser)))
	mux.Handle("GET /api/users/", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleGetUser)))
	mux.Handle("PUT /api/users/", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleUpdateUser)))
	mux.Handle("DELETE /api/users/", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleDeleteUser)))

	// Interface management
	mux.Handle("GET /api/interfaces", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleInterfaces))) // Using Config perms for now
	mux.Handle("GET /api/interfaces/available", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleAvailableInterfaces)))
	mux.Handle("POST /api/interfaces/update", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateInterface)))
	mux.Handle("POST /api/vlans", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleCreateVLAN)))
	mux.Handle("DELETE /api/vlans", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleDeleteVLAN)))
	mux.Handle("POST /api/bonds", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleCreateBond)))
	mux.Handle("DELETE /api/bonds", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleDeleteBond)))

	// Analytics
	analyticsHandlers := NewAnalyticsHandlers(s.client)
	mux.Handle("GET /api/analytics/bandwidth", s.require(storage.PermReadConfig, http.HandlerFunc(analyticsHandlers.HandleGetBandwidth)))
	mux.Handle("GET /api/analytics/top-talkers", s.require(storage.PermReadConfig, http.HandlerFunc(analyticsHandlers.HandleGetTopTalkers)))
	mux.Handle("GET /api/analytics/flows", s.require(storage.PermReadConfig, http.HandlerFunc(analyticsHandlers.HandleGetHistoricalFlows)))

	// Alerts
	mux.Handle("GET /api/alerts/history", s.require(storage.PermReadConfig, http.HandlerFunc(s.HandleGetAlertHistory)))
	mux.Handle("GET /api/alerts/rules", s.require(storage.PermReadConfig, http.HandlerFunc(s.HandleGetAlertRules)))
	mux.Handle("POST /api/alerts/rules", s.require(storage.PermWriteConfig, http.HandlerFunc(s.HandleUpdateAlertRule)))

	// Services
	mux.Handle("GET /api/services", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleServices)))
	mux.Handle("GET /api/leases", s.require(storage.PermReadDHCP, http.HandlerFunc(s.handleLeases)))
	mux.Handle("GET /api/topology", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleGetTopology)))
	mux.Handle("GET /api/network", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleGetNetworkDevices)))

	// Config Sections (CRUD handlers usually switch on method, so we keep generic path or would need to register GET/POST separately)
	mux.Handle("GET /api/config/policies", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetPolicies)))
	mux.Handle("POST /api/config/policies", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdatePolicies)))
	mux.Handle("GET /api/config/nat", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetNAT)))
	mux.Handle("POST /api/config/nat", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdateNAT)))
	mux.Handle("GET /api/config/ipsets", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetIPSets)))
	mux.Handle("POST /api/config/ipsets", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdateIPSets)))
	mux.Handle("GET /api/config/dhcp", s.require(storage.PermWriteDHCP, http.HandlerFunc(s.handleGetDHCP)))
	mux.Handle("POST /api/config/dhcp", s.require(storage.PermWriteDHCP, http.HandlerFunc(s.handleUpdateDHCP)))
	mux.Handle("GET /api/config/dns", s.require(storage.PermWriteDNS, http.HandlerFunc(s.handleGetDNS)))
	mux.Handle("POST /api/config/dns", s.require(storage.PermWriteDNS, http.HandlerFunc(s.handleUpdateDNS)))
	// DNS Query Logs
	mux.Handle("GET /api/dns/queries", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleGetDNSQueryHistory)))
	mux.Handle("GET /api/dns/stats", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleGetDNSStats)))

	// Previously updated handlers here...
	mux.Handle("GET /api/config/routes", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetRoutes)))
	mux.Handle("POST /api/config/routes", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateRoutes)))
	mux.Handle("GET /api/config/policy_routes", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetPolicyRoutes)))
	mux.Handle("POST /api/config/policy_routes", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdatePolicyRoutes)))
	mux.Handle("GET /api/config/zones", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetZones)))
	mux.Handle("POST /api/config/zones", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdateZones)))
	mux.Handle("GET /api/config/protections", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetProtections)))
	mux.Handle("POST /api/config/protections", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdateProtections)))
	mux.Handle("GET /api/config/qos", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleGetQoS)))
	mux.Handle("POST /api/config/qos", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleUpdateQoS)))
	mux.Handle("GET /api/config/scheduler", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetSchedulerConfig)))
	mux.Handle("POST /api/config/scheduler", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateSchedulerConfig)))
	mux.Handle("GET /api/config/vpn", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetVPN)))
	mux.Handle("POST /api/config/vpn", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateVPN)))
	mux.Handle("POST /api/vpn/import", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleImportVPNConfig)))

	// WireGuard API (key generation is stateless, other ops via config)
	mux.Handle("POST /api/wireguard/generate-key", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleWireGuardGenerateKey)))
	mux.Handle("GET /api/config/mark_rules", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetMarkRules)))
	mux.Handle("POST /api/config/mark_rules", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateMarkRules)))
	mux.Handle("GET /api/config/uid_routing", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleGetUIDRouting)))
	mux.Handle("POST /api/config/uid_routing", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateUIDRouting)))

	// Reordering
	mux.Handle("POST /api/policies/reorder", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handlePolicyReorder)))
	mux.Handle("POST /api/rules/reorder", s.require(storage.PermWriteFirewall, http.HandlerFunc(s.handleRuleReorder)))

	// ClearPath Policy Editor - Enriched Rules API
	rulesHandler := NewRulesHandler(s, s.statsCollector, s.deviceLookup)
	mux.Handle("GET /api/rules", s.require(storage.PermReadFirewall, http.HandlerFunc(rulesHandler.HandleGetRules)))
	mux.Handle("GET /api/rules/flat", s.require(storage.PermReadFirewall, http.HandlerFunc(rulesHandler.HandleGetFlatRules)))
	mux.Handle("GET /api/rules/groups", s.require(storage.PermReadFirewall, http.HandlerFunc(rulesHandler.HandleGetRuleGroups)))

	// Uplink Management
	uplinkAPI := NewUplinkAPI(s.client)
	mux.Handle("GET /api/uplinks/groups", s.require(storage.PermReadConfig, http.HandlerFunc(uplinkAPI.HandleGetGroups)))
	mux.Handle("POST /api/uplinks/switch", s.require(storage.PermWriteConfig, http.HandlerFunc(uplinkAPI.HandleSwitch)))
	mux.Handle("POST /api/uplinks/toggle", s.require(storage.PermWriteConfig, http.HandlerFunc(uplinkAPI.HandleToggle)))
	mux.Handle("POST /api/uplinks/test", s.require(storage.PermWriteConfig, http.HandlerFunc(uplinkAPI.HandleTest)))

	// Flow Management
	flowHandlers := NewFlowHandlers(s.client)
	mux.Handle("GET /api/flows", s.require(storage.PermReadFirewall, http.HandlerFunc(flowHandlers.HandleGetFlows)))
	mux.Handle("POST /api/flows/approve", s.require(storage.PermWriteFirewall, http.HandlerFunc(flowHandlers.HandleApprove)))
	mux.Handle("POST /api/flows/deny", s.require(storage.PermWriteFirewall, http.HandlerFunc(flowHandlers.HandleDeny)))
	mux.Handle("DELETE /api/flows", s.require(storage.PermWriteFirewall, http.HandlerFunc(flowHandlers.HandleDelete)))

	// System actions
	mux.Handle("POST /api/system/reboot", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleReboot)))
	mux.Handle("GET /api/system/backup", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleBackup)))
	mux.Handle("POST /api/system/restore", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleRestore)))
	mux.Handle("POST /api/system/wol", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleWakeOnLAN)))

	// Safe Mode (emergency lockdown)
	mux.Handle("GET /api/system/safe-mode", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleSafeModeStatus)))
	mux.Handle("POST /api/system/safe-mode", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleEnterSafeMode)))
	mux.Handle("DELETE /api/system/safe-mode", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleExitSafeMode)))

	// Scheduler
	mux.Handle("GET /api/scheduler/status", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleSchedulerStatus)))
	mux.Handle("POST /api/scheduler/run", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleSchedulerRun)))

	// HCL editing (Advanced mode)
	mux.Handle("GET /api/config/hcl", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleGetRawHCL)))
	mux.Handle("POST /api/config/hcl", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleUpdateRawHCL)))
	mux.Handle("GET /api/config/hcl/section", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleGetSectionHCL)))
	mux.Handle("POST /api/config/hcl/section", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleUpdateSectionHCL)))
	mux.Handle("POST /api/config/hcl/validate", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleValidateHCL)))
	mux.Handle("POST /api/config/hcl/save", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleSaveConfig)))

	// Backup management
	mux.Handle("GET /api/backups", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleBackups)))
	mux.Handle("POST /api/backups/create", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleCreateBackup)))
	mux.Handle("POST /api/backups/restore", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleRestoreBackup)))
	mux.Handle("GET /api/backups/content", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleBackupContent)))
	mux.Handle("POST /api/backups/pin", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handlePinBackup)))
	mux.Handle("GET /api/backups/settings", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleGetBackupSettings)))
	mux.Handle("POST /api/backups/settings", s.require(storage.PermAdminBackup, http.HandlerFunc(s.handleUpdateBackupSettings)))

	// Logging endpoints
	mux.Handle("GET /api/logs", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleLogs)))
	mux.Handle("GET /api/logs/sources", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleLogSources)))
	mux.Handle("GET /api/logs/stream", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleLogStream)))
	mux.Handle("GET /api/logs/stats", s.require(storage.PermReadLogs, http.HandlerFunc(s.handleLogStats)))

	// Audit log endpoint
	mux.Handle("GET /api/audit", s.require(storage.PermReadAudit, http.HandlerFunc(s.handleAuditQuery)))

	// Extended System Operations
	mux.Handle("GET /api/system/stats", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleSystemStats)))
	mux.Handle("GET /api/system/routes", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleSystemRoutes)))
	mux.Handle("GET /api/replication/status", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleReplicationStatus)))

	// Import Wizard
	mux.Handle("POST /api/import/upload", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleImportUpload)))
	mux.Handle("/api/import/", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleImportConfig)))

	// Debug endpoints (Admin only)
	mux.Handle("POST /api/debug/simulate-packet", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleSimulatePacket)))

	// Debug - Capture
	mux.Handle("POST /api/debug/capture", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleStartCapture)))
	mux.Handle("DELETE /api/debug/capture", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleStopCapture)))
	mux.Handle("GET /api/debug/capture/download", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleDownloadCapture)))
	mux.Handle("GET /api/debug/capture/status", s.require(storage.PermAdminSystem, http.HandlerFunc(s.handleGetCaptureStatus)))

	// IPSet management endpoints
	mux.Handle("GET /api/ipsets", s.require(storage.PermReadFirewall, http.HandlerFunc(s.handleIPSetList)))
	mux.Handle("/api/ipsets/", s.require(storage.PermReadFirewall, http.HandlerFunc(s.handleIPSetShow)))
	mux.Handle("GET /api/ipsets/cache/info", s.require(storage.PermReadFirewall, http.HandlerFunc(s.handleIPSetCacheInfo)))

	// Learning Engine
	mux.Handle("GET /api/runtime/containers", s.require(storage.PermReadConfig, http.HandlerFunc(s.getContainersHandler)))

	// Learning Engine
	mux.Handle("GET /api/learning/rules", s.require(storage.PermReadFirewall, http.HandlerFunc(s.handleLearningRules)))
	mux.Handle("/api/learning/rules/", s.require(storage.PermReadFirewall, http.HandlerFunc(s.handleLearningRule)))

	// Device Management
	mux.Handle("POST /api/devices/identity", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateDeviceIdentity)))
	mux.Handle("POST /api/devices/link", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleLinkMAC)))
	mux.Handle("POST /api/devices/unlink", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUnlinkMAC)))
	mux.Handle("GET /api/devices", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleGetDevices)))

	// Device Groups
	mux.Handle("GET /api/groups", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleGetDeviceGroups)))
	mux.Handle("POST /api/groups", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleUpdateDeviceGroup)))
	mux.Handle("DELETE /api/groups/", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleDeleteDeviceGroup)))

	// Staging & Diff
	mux.Handle("GET /api/config/diff", s.require(storage.PermReadConfig, http.HandlerFunc(s.handleGetConfigDiff)))
	mux.Handle("POST /api/config/discard", s.require(storage.PermWriteConfig, http.HandlerFunc(s.handleDiscardConfig)))
	mux.Handle("GET /api/config/pending-status", s.require(storage.PermReadConfig, http.HandlerFunc(s.handlePendingStatus)))

	mux.Handle("/metrics", promhttp.Handler())

	// Serve SPA static files with fallback to index.html
	if s.Assets != nil {
		mux.Handle("/", s.spaHandler(s.Assets, "index.html"))
	}
}

// syncConfig fetches the latest configuration from the control plane and updates the local cache
func (s *Server) syncConfig() error {
	if s.client == nil {
		return fmt.Errorf("control plane not connected")
	}

	cfg, err := s.client.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to fetch config: %v", err)
	}

	s.configMu.Lock()
	s.Config = cfg
	s.configMu.Unlock()

	s.logger.Info("Synchronized local config cache from control plane")
	return nil
}

// require middleware ensures the control plane client is available
func (s *Server) requireControlPlane(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.client == nil {
			WriteErrorCtx(w, r, http.StatusServiceUnavailable, "Control Plane Disconnected")
			return
		}
		next(w, r)
	}
}

// spaHandler serves static files, falling back to index.html for client-side routing.
//
// Security Analysis (Path Traversal): SAFE
// - Uses fs.FS interface which is confined to the embedded UI assets directory
// - assets.Open(path) cannot escape the fs.FS root regardless of ".." sequences
// - http.FileServer sanitizes paths before serving
// - Fallback to index.html only affects SPA routes, not arbitrary files
func (s *Server) spaHandler(assets fs.FS, fallback string) http.Handler {
	fileServer := http.FileServer(http.FS(assets))

	// Read index.html content once
	indexContent, err := fs.ReadFile(assets, fallback)
	if err != nil {
		logging.Error(fmt.Sprintf("Failed to read SPA fallback file %s: %v. Assets nil? %v", fallback, err, s.Assets == nil))
		indexContent = []byte("SPA Fallback Error")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prepare path (strip leading slash)
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// logging.APILog("debug", "SPA Handler: Request Path: '%s' -> '%s'", r.URL.Path, path)

		// Try to open the file to check if it exists
		f, err := assets.Open(path)
		if err == nil {
			stat, _ := f.Stat()
			isDir := stat.IsDir()
			f.Close()

			if !isDir {
				// File exists and is not a directory, serve it
				// logging.APILog("debug", "SPA Handler: File found: %s", path)
				fileServer.ServeHTTP(w, r)
				return
			}
			// If it's a directory, fall through to fallback behavior (unless it's an asset path)
		} else {
			// logging.APILog("debug", "SPA Handler: File not found: %s", path)
		}

		// Assert: File missing OR is directory.

		// If looking for static assets (js/css/images usually in _app or assets), return 404
		// to avoid serving HTML as JS (which causes syntax errors/redirect loops)
		if strings.HasPrefix(path, "_app/") || strings.HasPrefix(path, "assets/") {
			http.NotFound(w, r)
			return
		}

		// For everything else (SPA Routes/Directories), serve index.html directly
		// logging.APILog("debug", "SPA Handler: Serving fallback content for: %s", r.URL.Path)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Now(), bytes.NewReader(indexContent))
	})
}

// Handler returns the HTTP handler with security middleware applied
// Mitigation: OWASP A01:2021-Broken Access Control (CSRF prevention)
func (s *Server) Handler() http.Handler {
	// Apply CSRF middleware to protect against cross-site request forgery
	csrfMiddleware := CSRFMiddleware(s.csrfManager, s.authStore)

	// Chain: AccessLog -> CSRF -> i18n -> Mux
	return AccessLogger(csrfMiddleware(i18n.Middleware(s.mux)))
}

// Batch Request/Response types

// Start starts the HTTP server.
// Mitigation: OWASP A02:2021-Cryptographic Failures (TLS encryption)
func (s *Server) Start(addr string) error {
	// Create server with proper timeouts
	cfg := DefaultServerConfig()
	server := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	// Start metrics collector
	go s.collector.Start()

	// Update system metrics
	uptime := time.Since(s.startTime)
	s.collector.UpdateSystemMetrics(uptime)

	// Check if TLS is configured
	// TLS is enabled if tls_listen is set OR if tls_cert/tls_key are set
	tlsEnabled := s.Config != nil && s.Config.API != nil &&
		(s.Config.API.TLSListen != "" ||
			(s.Config.API.TLSCert != "" && s.Config.API.TLSKey != ""))

	if tlsEnabled {
		// Use default cert paths if not specified
		tlsCert := s.Config.API.TLSCert
		tlsKey := s.Config.API.TLSKey
		if tlsCert == "" {
			tlsCert = filepath.Join(brand.GetStateDir(), "certs", "server.crt")
		}
		if tlsKey == "" {
			tlsKey = filepath.Join(brand.GetStateDir(), "certs", "server.key")
		}

		// Auto-generate self-signed cert if missing
		// We use a 1 year validity for self-signed certs
		if _, err := tls.EnsureCertificate(tlsCert, tlsKey, 365); err != nil {
			logging.APILog("error", "Failed to ensure TLS certificate: %v", err)
			return err
		}

		// Determine TLS listen address (use TLSListen if set, otherwise addr)
		tlsAddr := addr
		if s.Config.API.TLSListen != "" {
			tlsAddr = s.Config.API.TLSListen
		}

		// Create TLS server
		tlsServer := &http.Server{
			Addr:              tlsAddr,
			Handler:           s.Handler(),
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		}

		// Start HTTP redirect server (unless disabled)
		if !s.Config.API.DisableHTTPRedirect {
			go s.startHTTPRedirectServer()
		}

		logging.APILog("info", "API server starting with TLS on %s", tlsAddr)
		return tlsServer.ListenAndServeTLS(tlsCert, tlsKey)
	}

	logging.APILog("info", "API server starting on %s (no TLS)", addr)
	return server.ListenAndServe()
}

// startHTTPRedirectServer starts a plain HTTP server that redirects to HTTPS.
func (s *Server) startHTTPRedirectServer() {
	// Determine listen address for HTTP redirect
	httpAddr := ":8080" // default
	if s.Config != nil && s.Config.API != nil && s.Config.API.Listen != "" {
		httpAddr = s.Config.API.Listen
	}

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build HTTPS URL - use port 8443 for internal redirect, or 443 for external
		target := "https://" + r.Host
		// If connecting to 8080, redirect to 8443
		// If connecting to 80 (DNATed), redirect to 443
		if strings.Contains(r.Host, ":8080") {
			target = strings.Replace(target, ":8080", ":8443", 1)
		} else if !strings.Contains(r.Host, ":") {
			// No port in Host header, assume standard ports (80->443)
			// Keep target as-is (will use default 443)
		}
		target += r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	redirectServer := &http.Server{
		Addr:              httpAddr,
		Handler:           redirectHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logging.APILog("info", "HTTP redirect server starting on %s -> HTTPS", httpAddr)
	if err := redirectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logging.APILog("error", "HTTP redirect server error: %v", err)
	}
}

// SetMetricsCollector injects the metrics collector
func (s *Server) SetMetricsCollector(c *metrics.Collector) {
	s.collector = c
}

// SetRuntimeService injects the runtime service
func (s *Server) SetRuntimeService(r *runtime.DockerClient) {
	s.runtime = r
}

// ServeListener starts the API server using an existing listener.
// This is used during seamless upgrades when the listener is handed off.
func (s *Server) ServeListener(listener net.Listener) error {
	cfg := DefaultServerConfig()
	server := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	go s.collector.Start()

	// Start learning service if enabled
	if s.learning != nil {
		if err := s.learning.Start(); err != nil {
			s.logger.Error("Failed to start learning service", "error", err)
		} else {
			s.logger.Info("Learning service started successfully")
		}
	}

	logging.APILog("info", "API server starting on handed-off listener %s", listener.Addr())
	return server.Serve(listener)
}

// loggingMiddleware logs all API requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := clock.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Log the request (skip static assets and metrics)
		if !strings.HasPrefix(r.URL.Path, "/assets/") && r.URL.Path != "/metrics" {
			level := "info"
			if wrapped.statusCode >= 400 {
				level = "warn"
			}
			if wrapped.statusCode >= 500 {
				level = "error"
			}
			logging.APILog(level, "%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, duration.Round(time.Millisecond))
		}
	})
}

// maxBodyMiddleware limits the size of request bodies to prevent memory exhaustion.
// Mitigation: OWASP A05:2021-Security Misconfiguration
func (s *Server) maxBodyMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip body limit for GET/HEAD/OPTIONS
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Check Content-Length header first (fast path)
			if r.ContentLength > maxBytes {
				http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
				return
			}

			// Wrap body with LimitReader for streaming requests
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Implement http.Flusher for SSE support
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Implement http.Hijacker for websocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijack not supported")
}

// require checks for sufficient permission from EITHER an API Key OR a User Session.
func (s *Server) require(perm storage.Permission, handler http.Handler) http.Handler {
	// Chain: handler -> audit -> CSRF -> auth check
	auditedHandler := s.auditMiddleware(handler)

	// Apply CSRF protection to the inner handler
	// The CSRF middleware itself handles skipping for API keys or non-state-changing methods
	protectedHandler := CSRFMiddleware(s.csrfManager, s.authStore)(auditedHandler)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass auth if not required by configuration (replicates legacy behavior)

		s.configMu.RLock()
		// s.logger.Info("require middleware: checking bypass")
		if s.Config != nil && s.Config.API != nil && !s.Config.API.RequireAuth {
			s.configMu.RUnlock()
			s.logger.Info("require middleware: bypassing auth")
			handler.ServeHTTP(w, r)
			return
		}
		s.configMu.RUnlock()

		// 1. API Key Check
		authHeader := r.Header.Get("Authorization")
		var apiKeyStr string
		if strings.HasPrefix(authHeader, "Bearer ") {
			// Could be Session Token OR API Key.
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(token, brand.APIKeyPrefixFull()) && s.apiKeyManager != nil {
				apiKeyStr = token
			}
		} else if strings.HasPrefix(authHeader, "ApiKey ") {
			apiKeyStr = strings.TrimPrefix(authHeader, "ApiKey ")
		} else {
			apiKeyStr = r.Header.Get("X-API-Key")
		}

		if apiKeyStr != "" && s.apiKeyManager != nil {
			key, err := s.apiKeyManager.ValidateKey(apiKeyStr)
			if err == nil {
				// Valid API Key found
				if key.HasPermission(perm) {
					// Success! Inject key into context
					ctx := WithAPIKey(r.Context(), key)
					protectedHandler.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Key exists but lacks permission
				writeAuthError(w, http.StatusForbidden, fmt.Sprintf("api key permission denied: %s required", perm))
				return
			}
			// Invalid key provided
			clientIP := getClientIP(r)
			logging.Error(fmt.Sprintf("Auth failed: invalid api key from %s", clientIP))

			// Record failed attempt for Fail2Ban-style blocking
			if s.security != nil {
				_ = s.security.RecordFailedAttempt(clientIP, "invalid_api_key", 5, 5*time.Minute)
			}

			writeAuthError(w, http.StatusUnauthorized, "invalid api key")
			return
		}

		if s.apiKeyManager == nil {
			logging.Error("Auth failed: apiKeyManager is nil")
		}

		// 2. User Session Check
		if s.authStore != nil {
			if cookie, err := r.Cookie("session"); err == nil {
				if user, err := s.authStore.ValidateSession(cookie.Value); err == nil {
					// Valid User Session found
					requiredRole := s.permToRole(perm)
					if user.Role.CanAccess(requiredRole) {
						// Success! Inject user into context
						ctx := context.WithValue(r.Context(), auth.UserContextKey, user)
						protectedHandler.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					writeAuthError(w, http.StatusForbidden, "user role insufficient")
					return
				}
			}
		}

		// 3. Fallback
		writeAuthError(w, http.StatusUnauthorized, "authentication required (api key or user session)")
	})
}

// permToRole maps fine-grained permissions to coarse-grained user roles.
// permToRole maps fine-grained permissions to coarse-grained user roles.
func (s *Server) permToRole(perm storage.Permission) string {
	// Admin permissions require Admin role
	if strings.HasPrefix(string(perm), "admin:") {
		return "admin"
	}

	// Write permissions require Operator role (modify)
	if strings.HasSuffix(string(perm), ":write") {
		return "modify"
	}

	// Default/Read permissions require Viewer role (view)
	return "view"
}
