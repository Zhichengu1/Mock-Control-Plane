// =============================================================================
// MOCK SONY VENDOR API
// =============================================================================
// This file simulates Sony's real device management API for testing purposes.
//
// WHY THIS EXISTS:
// - You can't test against real Sony servers during development
// - Real vendor APIs cost money, have rate limits, need credentials
// - This lets you develop and test locally without any external dependencies
//
// HOW IT'S USED:
// 1. Run this server on port 9000
// 2. SonyProvider sends HTTP requests here (thinking it's real Sony)
// 3. This server responds with realistic data
//
// IN PRODUCTION:
// - This server is NOT used
// - SonyProvider points to real Sony API (via SONY_API_URL env var)
// =============================================================================
package main

import (
	"encoding/json" // For JSON parsing - Sony API uses JSON
	"fmt"           // For string formatting
	"log"           // For logging requests (helpful for debugging)
	"math/rand"     // For generating random device IDs
	"net/http"      // For HTTP server
	"time"          // For timestamps in device IDs

	"github.com/Zhichengu1/mock-control-plane/pkg/models" // Sony data structures
	"github.com/gorilla/mux"                              // Router with URL params support
)

// devices is our in-memory "database" for this mock server.
// WHY A MAP: Simple key-value storage, device_id → device data
// WHY GLOBAL: All handlers need access to the same data
// NOTE: Data is lost when server restarts (that's fine for testing)
var devices = make(map[string]*models.SonyDeviceResponse)

// =============================================================================
// CREATE DEVICE HANDLER
// =============================================================================
// HandleCreateDevice simulates Sony's device creation endpoint.
//
// WHAT REAL SONY API WOULD DO:
// - Validate the request
// - Provision actual hardware
// - Return a device ID for future reference
//
// WHAT WE DO:
// - Validate the request (same as real)
// - Generate a fake device ID
// - Store in memory (instead of real hardware)
// - Return realistic response
func HandleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req models.SonyDeviceRequest

	// Decode JSON request
	// WHY: Convert incoming JSON bytes into Go struct
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// WHY 400: Client sent malformed JSON - their fault, not ours
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Validate required fields (device_name, model)
	// WHY: Real Sony API would reject requests missing required fields
	// We simulate the same behavior for realistic testing
	if req.DeviceName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "device_name is required"})
		return
	}
	if req.Model == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "model is required"})
		return
	}

	// Generate random device_id
	// WHY: Real Sony would assign an ID to the new device
	// This ID is used for all future operations (get, update, delete)
	deviceID := generateDeviceID()

	// Create device response
	// WHY "active": Simulates that device was successfully provisioned
	// Real Sony might return "provisioning" first, then "active" later
	deviceResponse := &models.SonyDeviceResponse{
		DeviceID: deviceID,
		Status:   "active",
		Message:  "Device provisioned successfully",
	}

	// Store in devices map
	// WHY: So we can retrieve/delete it later
	// Real Sony would store in their database
	devices[deviceID] = deviceResponse

	// WHY LOG: Helpful for debugging - see what requests came in
	log.Printf("Created device: %s (name: %s, model: %s)", deviceID, req.DeviceName, req.Model)

	// Return SonyDeviceResponse with status "active"
	// WHY 201 Created: REST convention for successful resource creation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(deviceResponse)
}

// =============================================================================
// GET DEVICE HANDLER
// =============================================================================
// HandleGetDevice simulates Sony's device retrieval endpoint.
//
// WHAT IT DOES:
// - Look up device by ID
// - Return current status
//
// WHY CONTROLLER CALLS THIS:
// - To refresh status (device might have gone offline)
// - To verify device still exists
// - To get latest metrics (bitrate, dropped frames, etc.)
func HandleGetDevice(w http.ResponseWriter, r *http.Request) {
	// Extract device_id from URL
	// WHY mux.Vars: Parses {id} from "/devices/{id}" route pattern
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "device ID is required"})
		return
	}

	// Look up in devices map
	// WHY: Check if device exists in our "database"
	device, exists := devices[deviceID]

	// Return 404 if not found
	// WHY 404: REST convention - resource doesn't exist
	// Controller will handle this and may mark resource as "Failed"
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "device not found"})
		return
	}

	// Return device details as JSON
	// WHY 200: Resource found and returned successfully
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(device)
}

// =============================================================================
// DELETE DEVICE HANDLER
// =============================================================================
// HandleDeleteDevice simulates Sony's deletion endpoint.
//
// WHAT REAL SONY WOULD DO:
// - Deprovision the hardware
// - Release any allocated resources
// - Remove from their database
//
// WHAT WE DO:
// - Just remove from our in-memory map
func HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	// Extract device_id from URL
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "device ID is required"})
		return
	}

	// Check if device exists before deleting
	// WHY CHECK: Some APIs return 404 for deleting non-existent resources
	// Others return 204 (idempotent). We chose 404 for clarity.
	if _, exists := devices[deviceID]; !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "device not found"})
		return
	}

	// Delete from devices map
	// WHY: Remove the device from our "database"
	delete(devices, deviceID)

	// WHY LOG: Track what was deleted for debugging
	log.Printf("Deleted device: %s", deviceID)

	// Return 204 No Content
	// WHY 204: REST convention - deletion successful, nothing to return
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// HEALTH CHECK HANDLER
// =============================================================================
// HandleHealthCheck simulates vendor health endpoint.
//
// WHY VENDORS HAVE THIS:
// - Clients need to know if API is reachable
// - Load balancers use this to route traffic
// - Monitoring systems use this for alerts
//
// WHAT CONTROLLER DOES WITH THIS:
// - Calls this periodically to check vendor connectivity
// - If fails, controller marks itself as unhealthy
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// WHY ALWAYS HEALTHY: This is a mock server, it's always "up"
	// Real Sony might check database connections, hardware status, etc.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// generateDeviceID creates a unique device identifier for Sony devices.
//
// FORMAT: "sony-dev-{unix_timestamp}-{random_4_digits}"
// EXAMPLE: "sony-dev-1706641234-0042"
//
// WHY THIS FORMAT:
// - "sony-dev-" prefix: Easy to identify as Sony device in logs
// - Unix timestamp: Rough ordering by creation time
// - Random suffix: Prevents collisions if multiple created same second
//
// NOTE: Real Sony would use their own ID format (maybe UUIDs)
func generateDeviceID() string {
	return fmt.Sprintf("sony-dev-%d-%04d", time.Now().Unix(), rand.Intn(10000))
}

// =============================================================================
// MAIN - MOCK SERVER ENTRY POINT
// =============================================================================
func main() {
	// Seed random number generator
	// WHY: So generateDeviceID() produces different IDs each run
	// Without this, you'd get the same "random" numbers every time
	rand.Seed(time.Now().UnixNano())

	// Set up HTTP router
	// WHY GORILLA MUX: Supports URL parameters like {id}
	r := mux.NewRouter()

	// Register routes - matching what real Sony API might look like
	// POST /devices      → Create new device
	// GET /devices/{id}  → Get device status
	// DELETE /devices/{id} → Delete device
	// GET /health        → Health check
	r.HandleFunc("/devices", HandleCreateDevice).Methods("POST")
	r.HandleFunc("/devices/{id}", HandleGetDevice).Methods("GET")
	r.HandleFunc("/devices/{id}", HandleDeleteDevice).Methods("DELETE")
	r.HandleFunc("/health", HandleHealthCheck).Methods("GET")

	// Start the server on port 9000
	// WHY 9000: Different from controller (8080) so both can run together
	// WHY log.Fatal: If server fails to start, exit with error
	log.Println("Mock Vendor API listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", r))
}
