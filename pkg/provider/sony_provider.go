package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Zhichengu1/mock-control-plane/pkg/client"
	"github.com/Zhichengu1/mock-control-plane/pkg/models"
)

// =============================================================================
// SONY PROVIDER
// =============================================================================
// SonyProvider implements the VendorProvider interface for Sony device
// integration. It handles all communication with Sony's device management API,
// transforming Forge's internal resource model to Sony's expected format.
//
// Responsibilities:
// 1. Transform ForgeResource ↔ SonyDeviceRequest/Response
// 2. Execute HTTP requests to Sony API with proper authentication
// 3. Handle errors and map Sony statuses to Forge phases
// 4. Implement retry logic for transient failures
//
// Thread Safety: This struct is safe for concurrent use. The HTTPClient
// is shared but http.Client is safe for concurrent use.
// =============================================================================

// SonyProvider implements VendorProvider for Sony device management.
// It encapsulates all Sony-specific API communication logic.
type SonyProvider struct {
	// BaseURL is the root URL for Sony's API (e.g., "https://api.sony.example.com/v1")
	BaseURL string

	// APIKey is the authentication key for Sony API requests.
	// Sent in the Authorization header as "Bearer <APIKey>"
	APIKey string

	// HTTPClient is a reusable HTTP client with connection pooling.
	// Using a shared client improves performance through connection reuse.
	HTTPClient *http.Client
}

// NewSonyProvider creates a new SonyProvider instance with the given configuration.
// It initializes the HTTP client with sensible defaults for API communication.
//
// Parameters:
//   - baseURL: The Sony API base URL (e.g., "https://api.sony.example.com")
//   - apiKey:  The API key for authentication
//
// Returns:
//   - A configured SonyProvider ready for use
//
// Example:
//
//	provider := NewSonyProvider("https://api.sony.example.com", "secret-key")
func NewSonyProvider(baseURL, apiKey string) *SonyProvider {
	return &SonyProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			// Timeout prevents hanging on slow/unresponsive servers.
			// 30 seconds is generous for most API calls.
			Timeout: 30 * time.Second,
		},
	}
}

// =============================================================================
// CREATE OPERATION
// =============================================================================

// Create provisions a new device in Sony's system based on the ForgeResource spec.
// This is the primary method for creating new resources.
//
// Flow:
// 1. Transform ForgeResource → SonyDeviceRequest (extract relevant fields)
// 2. Marshal the request to JSON
// 3. Create HTTP POST request to Sony's /devices endpoint
// 4. Add authentication headers
// 5. Execute request with retry logic for transient failures
// 6. Parse SonyDeviceResponse
// 7. Transform response → ResourceStatus
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - resource: The ForgeResource containing desired state configuration
//
// Returns:
//   - *models.ResourceStatus: The observed state after creation
//   - error: Any error encountered during the operation
//
// Error Handling:
//   - Returns error if JSON marshaling fails
//   - Returns error if HTTP request fails after retries
//   - Returns error with status code details if Sony returns non-2xx
//   - Returns error if response parsing fails
func (s *SonyProvider) Create(ctx context.Context, resource *models.ForgeResource) (*models.ResourceStatus, error) {
	// =========================================================================
	// STEP 1: Transform ForgeResource → SonyDeviceRequest
	// =========================================================================
	// We need to convert Forge's generic resource model into Sony's specific
	// API format. This involves:
	// - Mapping common fields (name → device_name)
	// - Extracting vendor-specific config values
	// - Building Sony-specific nested structures
	// =========================================================================
	sonyRequest := s.buildSonyRequest(resource)

	// =========================================================================
	// STEP 2: Marshal request to JSON
	// =========================================================================
	// Convert the Go struct to JSON bytes for the HTTP body.
	// We use json.Marshal which returns compact JSON (no extra whitespace).
	// =========================================================================
	requestBody, err := json.Marshal(sonyRequest)
	if err != nil {
		// This typically indicates a programming error (unencodable types),
		// not a runtime issue. Wrap with context for debugging.
		return nil, fmt.Errorf("failed to marshal Sony request: %w", err)
	}

	// =========================================================================
	// STEP 3: Create HTTP POST request
	// =========================================================================
	// Build the HTTP request to Sony's device creation endpoint.
	// We use bytes.NewBuffer to create an io.Reader from our JSON bytes.
	// The context is attached to the request for cancellation support.
	// =========================================================================
	url := s.BaseURL + "/devices"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		// This error is rare - typically only happens with malformed URLs
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// =========================================================================
	// STEP 4: Add required headers
	// =========================================================================
	// Set Content-Type to indicate we're sending JSON.
	// Set Authorization with our API key for authentication.
	// Some APIs may require additional headers (X-Request-ID, etc.)
	// =========================================================================
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Accept", "application/json")
	// Optional: Add request tracing header for debugging
	req.Header.Set("X-Forge-Resource-ID", resource.ID)

	// =========================================================================
	// STEP 5: Execute request with retry logic
	// =========================================================================
	// Use the retry wrapper to handle transient failures.
	// Retries are performed for:
	// - Network errors (connection refused, timeout)
	// - 5xx server errors (internal error, bad gateway, etc.)
	// Retries are NOT performed for:
	// - 4xx client errors (bad request, unauthorized, not found)
	// =========================================================================
	resp, err := client.DoWithRetry(ctx, req, 3) // 3 retries = 4 total attempts
	if err != nil {
		return nil, fmt.Errorf("failed to execute Sony API request: %w", err)
	}
	// Always close the response body to prevent resource leaks.
	// Using defer ensures cleanup even if later code panics.
	defer resp.Body.Close()

	// =========================================================================
	// STEP 6: Read and validate response
	// =========================================================================
	// Read the full response body for parsing.
	// We read completely before checking status code so we can include
	// error details in our error message.
	// =========================================================================
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Sony API response: %w", err)
	}

	// Check for non-success status codes
	// 201 Created is the expected success code for resource creation
	// We also accept 200 OK as some APIs use that instead
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sony API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// =========================================================================
	// STEP 7: Parse Sony's response
	// =========================================================================
	// Unmarshal the JSON response into our Go struct.
	// This extracts the device_id and status we need.
	// =========================================================================
	var sonyResponse models.SonyDeviceResponse
	if err := json.Unmarshal(respBody, &sonyResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Sony API response: %w", err)
	}

	// =========================================================================
	// STEP 8: Transform response → ResourceStatus
	// =========================================================================
	// Map Sony's status values to Forge's phase values.
	// This abstraction allows the controller to work uniformly
	// across different vendors.
	// =========================================================================
	status := s.buildResourceStatus(&sonyResponse)

	return status, nil
}

// buildSonyRequest transforms a ForgeResource into a SonyDeviceRequest.
// This is where the vendor-specific mapping logic lives.
//
// Mapping Rules:
// - resource.Name → DeviceName
// - resource.Spec.Config["sony_model"] → Model
// - resource.Spec.Resolution/Bitrate/etc → StreamConfig
// - resource.Spec.Config (other keys) → Settings
func (s *SonyProvider) buildSonyRequest(resource *models.ForgeResource) *models.SonyDeviceRequest {
	// Initialize the request with basic fields
	request := &models.SonyDeviceRequest{
		DeviceName: resource.Name,
		Model:      s.extractStringConfig(resource, "sony_model", "HDC-5500"), // Default model
		Settings:   make(map[string]string),
		Metadata: map[string]string{
			"forge_id":        resource.ID,
			"forge_namespace": resource.Namespace,
			"forge_type":      resource.Type,
		},
	}

	// Extract IP address if configured
	if ip := s.extractStringConfig(resource, "ip_address", ""); ip != "" {
		request.IPAddress = ip
	}

	// Extract port if configured
	if port := s.extractIntConfig(resource, "port", 0); port > 0 {
		request.Port = port
	}

	// Build Settings map from common Forge fields
	// These are normalized settings that map to Sony's API
	if resource.Spec.Resolution != "" {
		request.Settings["resolution"] = s.mapResolutionToSony(resource.Spec.Resolution)
	}
	if resource.Spec.FrameRate > 0 {
		request.Settings["frame_rate"] = fmt.Sprintf("%.2f", resource.Spec.FrameRate)
	}
	if resource.Spec.Codec != "" {
		request.Settings["codec"] = resource.Spec.Codec
	}

	// Build StreamConfig if streaming is configured
	if resource.Spec.StreamURL != "" {
		request.StreamConfig = &models.SonyStreamConfig{
			Enabled:        true,
			Protocol:       s.detectStreamProtocol(resource.Spec.StreamURL),
			DestinationURL: resource.Spec.StreamURL,
			Resolution:     s.mapResolutionToSony(resource.Spec.Resolution),
			Bitrate:        int(resource.Spec.Bitrate / 1000), // Convert bps → kbps
			FrameRate:      resource.Spec.FrameRate,
			Codec:          s.mapCodecToSony(resource.Spec.Codec),
			LatencyMode:    s.mapLatencyModeToSony(resource.Spec.LatencyMode),
		}
	}

	// Build RecordingConfig if recording is enabled
	if resource.Spec.RecordingEnabled {
		request.RecordingConfig = &models.SonyRecordingConfig{
			Enabled:       true,
			StoragePath:   resource.Spec.RecordingPath,
			Format:        s.extractStringConfig(resource, "recording_format", "MXF"),
			Quality:       s.extractStringConfig(resource, "recording_quality", "production"),
			RetentionDays: resource.Spec.RetentionDays,
		}
	}

	// Build NetworkConfig if network settings are specified
	if vlan := s.extractIntConfig(resource, "vlan_id", 0); vlan > 0 {
		request.NetworkConfig = &models.SonyNetworkConfig{
			PrimaryInterface: s.extractStringConfig(resource, "network_interface", "eth0"),
			VLANID:           vlan,
			MTU:              s.extractIntConfig(resource, "mtu", 1500),
		}
	}

	// Build TallyConfig if tally is configured
	if tallyEnabled := s.extractBoolConfig(resource, "tally_enabled"); tallyEnabled {
		request.TallyConfig = &models.SonyTallyConfig{
			Enabled:         true,
			Color:           s.extractStringConfig(resource, "tally_color", "red"),
			ControlProtocol: s.extractStringConfig(resource, "tally_protocol", "TSL"),
			ControlAddress:  s.extractStringConfig(resource, "tally_address", ""),
		}
	}

	return request
}

// buildResourceStatus transforms a SonyDeviceResponse into a ResourceStatus.
// This maps Sony's status terminology to Forge's standardized phases.
//
// Sony Status → Forge Phase mapping:
// - "active"       → "Running"
// - "inactive"     → "Pending"
// - "provisioning" → "Provisioning"
// - "error"        → "Failed"
// - "maintenance"  → "Updating"
// - (unknown)      → "Unknown"
func (s *SonyProvider) buildResourceStatus(response *models.SonyDeviceResponse) *models.ResourceStatus {
	status := &models.ResourceStatus{
		VendorID: response.DeviceID,
		Message:  response.Message,
	}

	// Map Sony status to Forge phase
	switch response.Status {
	case "active":
		status.Phase = "Running"
		status.HealthStatus = "healthy"
	case "inactive":
		status.Phase = "Pending"
		status.HealthStatus = "unknown"
	case "provisioning":
		status.Phase = "Provisioning"
		status.HealthStatus = "unknown"
	case "error":
		status.Phase = "Failed"
		status.HealthStatus = "unhealthy"
		status.ErrorCount++
	case "maintenance":
		status.Phase = "Updating"
		status.HealthStatus = "degraded"
	default:
		status.Phase = "Unknown"
		status.HealthStatus = "unknown"
	}

	status.LastHealthCheck = time.Now()
	status.LastSuccessfulOperation = time.Now()

	// Extract streaming metrics if available
	if response.StreamStatus != nil {
		status.CurrentBitrate = int64(response.StreamStatus.CurrentBitrate) * 1000 // kbps → bps
		status.DroppedFrames = response.StreamStatus.DroppedFrames
		status.ConnectionCount = response.StreamStatus.ViewerCount
		if response.StreamStatus.UptimeSeconds > 0 {
			status.Uptime = time.Duration(response.StreamStatus.UptimeSeconds) * time.Second
		}
	}

	return status
}

// =============================================================================
// READ OPERATION
// =============================================================================

// Read retrieves the current state of a device from Sony's system.
// This is used for status synchronization and health checks.
//
// Flow:
// 1. Create HTTP GET request to /devices/{vendorID}
// 2. Add authentication headers
// 3. Execute request with retry logic
// 4. Parse response and build ResourceStatus
//
// Parameters:
//   - ctx: Context for cancellation
//   - vendorID: The Sony device ID (from ResourceStatus.VendorID)
//
// Returns:
//   - *models.ResourceStatus: Current observed state
//   - error: Any error encountered
func (s *SonyProvider) Read(ctx context.Context, vendorID string) (*models.ResourceStatus, error) {
	// =========================================================================
	// STEP 1: Build the request URL
	// =========================================================================
	// Sony's API expects device retrieval at /devices/{device_id}
	// =========================================================================
	url := s.BaseURL + "/devices/" + vendorID

	// =========================================================================
	// STEP 2: Create HTTP GET request
	// =========================================================================
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// =========================================================================
	// STEP 3: Add authentication headers
	// =========================================================================
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Accept", "application/json")

	// =========================================================================
	// STEP 4: Execute request with retry logic
	// =========================================================================
	resp, err := client.DoWithRetry(ctx, req, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Sony API request: %w", err)
	}
	defer resp.Body.Close()

	// =========================================================================
	// STEP 5: Handle response
	// =========================================================================
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Sony API response: %w", err)
	}

	// Handle 404 Not Found - device may have been deleted externally
	if resp.StatusCode == http.StatusNotFound {
		return &models.ResourceStatus{
			Phase:        "Failed",
			Message:      "Device not found in Sony system",
			VendorID:     vendorID,
			HealthStatus: "unhealthy",
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sony API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// =========================================================================
	// STEP 6: Parse response and build status
	// =========================================================================
	var sonyResponse models.SonyDeviceResponse
	if err := json.Unmarshal(respBody, &sonyResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Sony API response: %w", err)
	}

	return s.buildResourceStatus(&sonyResponse), nil
}

// =============================================================================
// UPDATE OPERATION
// =============================================================================

// Update modifies an existing device in Sony's system.
// This is called when the user changes the ForgeResource spec.
//
// Flow:
// 1. Transform ForgeResource → SonyDeviceRequest
// 2. Create HTTP PATCH request to /devices/{vendorID}
// 3. Execute and parse response
//
// Note: Some vendors use PUT (full replacement) vs PATCH (partial update).
// Sony's API uses PATCH for partial updates.
//
// Parameters:
//   - ctx: Context for cancellation
//   - resource: The updated ForgeResource with new spec
//
// Returns:
//   - *models.ResourceStatus: State after update
//   - error: Any error encountered
func (s *SonyProvider) Update(ctx context.Context, resource *models.ForgeResource) (*models.ResourceStatus, error) {
	// =========================================================================
	// STEP 1: Validate we have a vendor ID to update
	// =========================================================================
	if resource.Status.VendorID == "" {
		return nil, fmt.Errorf("cannot update resource without vendor ID")
	}

	// =========================================================================
	// STEP 2: Build the update request
	// =========================================================================
	sonyRequest := s.buildSonyRequest(resource)

	requestBody, err := json.Marshal(sonyRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Sony request: %w", err)
	}

	// =========================================================================
	// STEP 3: Create HTTP PATCH request
	// =========================================================================
	// PATCH is used for partial updates - only provided fields are changed.
	// PUT would require sending all fields and would overwrite unspecified
	// fields with defaults.
	// =========================================================================
	url := s.BaseURL + "/devices/" + resource.Status.VendorID
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forge-Resource-ID", resource.ID)

	// =========================================================================
	// STEP 4: Execute with retries
	// =========================================================================
	resp, err := client.DoWithRetry(ctx, req, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Sony API request: %w", err)
	}
	defer resp.Body.Close()

	// =========================================================================
	// STEP 5: Parse response
	// =========================================================================
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Sony API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sony API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var sonyResponse models.SonyDeviceResponse
	if err := json.Unmarshal(respBody, &sonyResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Sony API response: %w", err)
	}

	return s.buildResourceStatus(&sonyResponse), nil
}

// =============================================================================
// DELETE OPERATION
// =============================================================================

// Delete removes a device from Sony's system.
// This is called when the user deletes the ForgeResource.
//
// Flow:
// 1. Create HTTP DELETE request to /devices/{vendorID}
// 2. Execute request
// 3. Handle response (204 No Content is success)
//
// Note: Delete operations are idempotent - deleting an already-deleted
// resource should not return an error (404 is handled gracefully).
//
// Parameters:
//   - ctx: Context for cancellation
//   - vendorID: The Sony device ID to delete
//
// Returns:
//   - error: Any error encountered (nil on success)
func (s *SonyProvider) Delete(ctx context.Context, vendorID string) error {
	// =========================================================================
	// STEP 1: Create HTTP DELETE request
	// =========================================================================
	url := s.BaseURL + "/devices/" + vendorID
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	// =========================================================================
	// STEP 2: Execute with retries
	// =========================================================================
	resp, err := client.DoWithRetry(ctx, req, 3)
	if err != nil {
		return fmt.Errorf("failed to execute Sony API request: %w", err)
	}
	defer resp.Body.Close()

	// =========================================================================
	// STEP 3: Handle response
	// =========================================================================
	// 204 No Content - successful deletion
	// 200 OK - some APIs return this with a body
	// 404 Not Found - already deleted, treat as success (idempotent)
	// =========================================================================
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK, http.StatusNotFound:
		return nil // Success (or already deleted)
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Sony API returned status %d: %s", resp.StatusCode, string(respBody))
	}
}

// =============================================================================
// HEALTH CHECK OPERATION
// =============================================================================

// HealthCheck verifies connectivity to Sony's API.
// This is used by the controller to detect vendor API outages.
//
// Flow:
// 1. Send GET request to /health endpoint
// 2. Check for 200 OK response
//
// Note: This checks API connectivity, not individual device health.
// For device health, use Read() and check the HealthStatus field.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: nil if healthy, error describing the issue otherwise
func (s *SonyProvider) HealthCheck(ctx context.Context) error {
	// =========================================================================
	// STEP 1: Create health check request
	// =========================================================================
	url := s.BaseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	// =========================================================================
	// STEP 2: Execute request (no retries for health check)
	// =========================================================================
	// We use the HTTPClient directly instead of DoWithRetry because:
	// - Health checks should be fast
	// - Retries would hide transient issues
	// - We want immediate feedback on connectivity
	// =========================================================================
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sony API health check failed: %w", err)
	}
	defer resp.Body.Close()

	// =========================================================================
	// STEP 3: Validate response
	// =========================================================================
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Sony API unhealthy (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================
// These private methods provide common functionality used across operations.
// They handle configuration extraction and format conversion.
// =============================================================================

// extractStringConfig safely extracts a string value from the Config map.
// Returns the defaultValue if the key doesn't exist or isn't a string.
func (s *SonyProvider) extractStringConfig(resource *models.ForgeResource, key string, defaultValue string) string {
	if resource.Spec.Config == nil {
		return defaultValue
	}
	if val, ok := resource.Spec.Config[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

// extractIntConfig safely extracts an integer value from the Config map.
// Handles both int and float64 (JSON numbers decode as float64).
func (s *SonyProvider) extractIntConfig(resource *models.ForgeResource, key string, defaultValue int) int {
	if resource.Spec.Config == nil {
		return defaultValue
	}
	if val, ok := resource.Spec.Config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// extractBoolConfig safely extracts a boolean value from the Config map.
func (s *SonyProvider) extractBoolConfig(resource *models.ForgeResource, key string) bool {
	if resource.Spec.Config == nil {
		return false
	}
	if val, ok := resource.Spec.Config[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// mapResolutionToSony converts Forge resolution names to Sony's format.
// Forge uses friendly names; Sony uses pixel dimensions.
func (s *SonyProvider) mapResolutionToSony(resolution string) string {
	switch resolution {
	case "SD", "480p":
		return "720x480"
	case "HD", "720p":
		return "1280x720"
	case "FHD", "1080p":
		return "1920x1080"
	case "4K", "2160p", "UHD":
		return "3840x2160"
	case "8K", "4320p":
		return "7680x4320"
	default:
		return resolution // Pass through if already in pixel format
	}
}

// mapCodecToSony converts Forge codec names to Sony's format.
func (s *SonyProvider) mapCodecToSony(codec string) string {
	switch codec {
	case "H.265/HEVC", "HEVC":
		return "H.265"
	default:
		return codec // Most codecs use the same name
	}
}

// mapLatencyModeToSony converts Forge latency modes to Sony's format.
func (s *SonyProvider) mapLatencyModeToSony(mode string) string {
	switch mode {
	case "low":
		return "ultra_low"
	case "normal":
		return "low" // Sony's "low" is closer to our "normal"
	case "high":
		return "normal"
	default:
		return "low"
	}
}

// detectStreamProtocol determines the streaming protocol from a URL.
// This is used when the user provides a stream URL without explicit protocol config.
func (s *SonyProvider) detectStreamProtocol(url string) string {
	switch {
	case len(url) >= 7 && url[:7] == "rtmp://":
		return "RTMP"
	case len(url) >= 6 && url[:6] == "srt://":
		return "SRT"
	case len(url) >= 7 && url[:7] == "rtsp://":
		return "RTSP"
	case len(url) >= 6 && url[:6] == "ndi://":
		return "NDI"
	default:
		return "RTMP" // Default to RTMP
	}
}
