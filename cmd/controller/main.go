// =============================================================================
// FORGE CONTROLLER - REST API SERVER
// =============================================================================
// This is the main entry point for the Forge Orchestrator. It provides a
// unified REST API that abstracts multiple vendor systems (Sony, AWS, etc.)
// behind a single interface.
//
// WHY THIS EXISTS:
// - Different vendors have different APIs (Sony uses REST, AWS uses SDK, etc.)
// - Without abstraction, every app would need vendor-specific code
// - This controller provides ONE API that works with ALL vendors
//
// HOW IT WORKS:
// 1. Client sends request to this server (port 8080)
// 2. Controller picks the right provider based on vendor_type
// 3. Provider translates and sends request to vendor
// 4. Controller stores result and returns response
// =============================================================================
package main

import (
	"context"       
	"encoding/json" 
	"fmt"           
	"log"           
	"net/http"     
	"os"            
	"sync"          
	"time"          
	"github.com/Zhichengu1/mock-control-plane/pkg/models"   // Our data structures
	"github.com/Zhichengu1/mock-control-plane/pkg/provider" // Vendor translators
	"github.com/gorilla/mux"                                // Router - better than default, supports URL params like /resources/{id}
)

type Controller struct {
	Providers  map[string]provider.VendorProvider // "sony" → SonyProvider, "aws" → AWSProvider
	ResourceDB map[string]*models.ForgeResource   // "res-123" → resource data
	mu         sync.RWMutex                       // Protects ResourceDB from concurrent access
}

func NewController() *Controller {
	sonyBaseURL := os.Getenv("SONY_API_URL")
	sonyAPIKey := os.Getenv("SONY_API_KEY")

	if sonyBaseURL == "" {
		sonyBaseURL = "http://localhost:9000" // Our mock Sony server
	}
	if sonyAPIKey == "" {
		sonyAPIKey = "test-api-key" // Fake key for testing
	}

	return &Controller{
		Providers: map[string]provider.VendorProvider{
			"sony": provider.NewSonyProvider(sonyBaseURL, sonyAPIKey),
		},
		// Initialize empty database
		// WHY make(): In Go, maps must be initialized before use
		ResourceDB: make(map[string]*models.ForgeResource),
	}
}

func (c *Controller) HandleCreateResource(w http.ResponseWriter, r *http.Request) {
	var resource models.ForgeResource

	// Step 1: Decode the JSON request body
	// WHY: Convert raw JSON bytes into a Go struct we can work with
	// WHY NewDecoder: Streams directly from request body, efficient for large payloads
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		// WHY 400 Bad Request: Client sent invalid data, not our fault
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return // WHY return: Stop processing, don't continue with bad data
	}

	// Step 2: Validate required fields
	// WHY VALIDATE: Catch errors early before we do expensive vendor API calls
	// WHY THESE FIELDS: Minimum info needed to create any resource
	if resource.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "name is required"})
		return
	}
	if resource.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "type is required"})
		return
	}
	if resource.Spec.VendorType == "" {
		// WHY vendor_type required: We need to know WHICH provider to use
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "vendor_type is required"})
		return
	}

	// Step 3: Generate a unique ID for this resource
	// WHY WE GENERATE IT: Client doesn't control IDs, prevents duplicates/conflicts
	// WHY NOT UUID: Nanosecond timestamp is simpler, good enough for this project
	resource.ID = generateResourceID()

	// Step 4: Set timestamps
	// WHY: Track when resource was created for auditing/debugging
	// WHY BOTH SAME: At creation time, created and updated are identical
	resource.CreatedAt = time.Now()
	resource.UpdatedAt = time.Now()

	// Step 5: Initialize the resource status to "Pending"
	// WHY "Pending": Resource exists but vendor hasn't confirmed yet
	// This follows Kubernetes-style status patterns
	resource.Status.Phase = "Pending"
	resource.Status.Message = "Resource creation initiated"

	// Step 6: Select the provider based on resource.Spec.VendorType
	// WHY MAP LOOKUP: O(1) lookup, easy to add new vendors
	// This is the key abstraction - controller doesn't know vendor details
	selectedProvider, exists := c.Providers[resource.Spec.VendorType]
	if !exists {
		// WHY 400: Client asked for a vendor we don't support
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported vendor: " + resource.Spec.VendorType})
		return
	}

	// Step 7: Create a context with timeout for the vendor API call
	// WHY CONTEXT: Provides cancellation and timeout capabilities
	// WHY 30 SECONDS: Generous timeout for slow vendor APIs
	// WHY defer cancel(): Prevents goroutine/memory leaks if we return early
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 8: Call provider.Create() with the context and resource
	// WHY PROVIDER: Provider handles all vendor-specific translation and HTTP calls
	// Controller doesn't know HOW to talk to Sony - provider does
	status, err := selectedProvider.Create(ctx, &resource)
	if err != nil {
		// WHY NOT RETURN ERROR: We still want to save the failed resource
		// so users can query it and see what went wrong
		resource.Status.Phase = "Failed"
		resource.Status.Message = "Vendor API error: " + err.Error()
		log.Printf("Failed to create resource with vendor: %v", err)
	} else {
		// WHY COPY STATUS: Provider returns the observed state from vendor
		// This includes VendorID which we need for future Read/Update/Delete
		resource.Status = *status
	}

	// Step 9: Store the resource in the in-memory database
	// WHY LOCK: Multiple requests might try to write at the same time
	// Without lock, we could corrupt the map (race condition)
	c.mu.Lock()
	c.ResourceDB[resource.ID] = &resource
	c.mu.Unlock() // WHY UNLOCK IMMEDIATELY: Don't hold lock during JSON encoding

	// Step 10: Return the created resource as JSON with HTTP 201
	// WHY 201 Created: REST convention - resource was successfully created
	// WHY Content-Type: Tells client to parse response as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resource)
}


func (c *Controller) HandleGetResource(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract the resource ID from the URL path
	// WHY mux.Vars: Gorilla mux extracts {id} from "/resources/{id}" pattern
	vars := mux.Vars(r)
	resourceID := vars["id"]
	if resourceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource ID is required"})
		return
	}

	// Step 2: Look up the resource from the in-memory database
	// WHY RLock (not Lock): Read lock allows multiple simultaneous readers
	// Only blocks if someone is writing. Better performance for read-heavy workloads.
	c.mu.RLock()
	resource, exists := c.ResourceDB[resourceID]
	c.mu.RUnlock() // WHY UNLOCK BEFORE CHECK: Don't hold lock while doing other work

	if !exists {
		// WHY 404: REST convention - resource doesn't exist
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource not found"})
		return
	}

	// Step 3: Get the vendor type from the stored resource
	vendorType := resource.Spec.VendorType

	// Step 4: Select the appropriate provider
	selectedProvider, exists := c.Providers[vendorType]
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "provider not configured"})
		return
	}

	// Step 5: Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Step 6: Call provider.Read() to get current status from vendor
	// WHY CHECK VendorID: If empty, resource was never created in vendor system
	// (maybe creation failed). Can't read something that doesn't exist.
	if resource.Status.VendorID != "" {
		status, err := selectedProvider.Read(ctx, resource.Status.VendorID)
		if err != nil {
			// WHY NOT FAIL: Vendor being down shouldn't break our API
			// GRACEFUL DEGRADATION: Return stale cache data instead of error
			log.Printf("Failed to read from vendor: %v", err)
		} else {
			// Update the resource with fresh status from vendor
			// WHY UPDATE: Vendor status may have changed (device went offline, etc.)
			resource.Status = *status
			resource.UpdatedAt = time.Now()
			// Update in database so next read doesn't need vendor call
			c.mu.Lock()
			c.ResourceDB[resourceID] = resource
			c.mu.Unlock()
		}
	}

	// Step 7: Return the resource as JSON with HTTP 200
	// WHY 200 OK: Resource found and returned (even if using cached data)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resource)
}


func (c *Controller) HandleDeleteResource(w http.ResponseWriter, r *http.Request) {
	// Step 1: Extract resource ID from URL
	vars := mux.Vars(r)
	resourceID := vars["id"]

	if resourceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource ID is required"})
		return
	}

	// Step 2: Look up the resource to get vendor information
	// WHY LOOKUP FIRST: Need VendorID to tell vendor what to delete
	c.mu.RLock()
	resource, exists := c.ResourceDB[resourceID]
	c.mu.RUnlock()

	if !exists {
		// WHY 404: Can't delete something that doesn't exist
		// Note: Some APIs return 204 for "already deleted" (idempotent)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource not found"})
		return
	}

	// Step 3: Select the provider
	selectedProvider, exists := c.Providers[resource.Spec.VendorType]
	if !exists {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "provider not configured"})
		return
	}

	// Step 4: Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 5: Call provider.Delete() with the vendor ID
	// WHY CHECK VendorID: If empty, nothing exists in vendor system to delete
	if resource.Status.VendorID != "" {
		err := selectedProvider.Delete(ctx, resource.Status.VendorID)
		if err != nil {
			// WHY 500: Vendor delete failed - could be network, auth, etc.
			// WHY RETURN (not continue): Don't delete locally if vendor failed
			// This maintains consistency - resource still exists in vendor
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete from vendor: " + err.Error()})
			return
		}
	}

	// Step 6: Remove from in-memory database
	// WHY AFTER VENDOR: Only delete locally after vendor confirms deletion
	c.mu.Lock()
	delete(c.ResourceDB, resourceID) // Built-in Go function to remove map entry
	c.mu.Unlock()

	// Step 7: Return HTTP 204 No Content (successful deletion)
	// WHY 204 (not 200): REST convention - success but no body to return
	// The resource no longer exists, so there's nothing to return
	w.WriteHeader(http.StatusNoContent)
}

// generateResourceID creates a unique resource identifier.
//
// WHY TIME-BASED:
// - Simple and doesn't require external dependencies
// - Nanoseconds gives uniqueness for reasonable request rates
// - Prefix "res-" makes IDs human-readable and identifiable
//
// LIMITATION: Could produce duplicates under extreme concurrency.
// For production, consider using UUID: github.com/google/uuid
func generateResourceID() string {
	return fmt.Sprintf("res-%d", time.Now().UnixNano())
}


func (c *Controller) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// WHY SHORT TIMEOUT: Health checks should be fast
	// If vendor takes > 5 seconds, something is wrong
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthy := true
	// Check each registered provider
	for name, selectedProvider := range c.Providers {
		if err := selectedProvider.HealthCheck(ctx); err != nil {
			// WHY LOG: Operators need to know which provider failed
			log.Printf("Provider %s unhealthy: %v", name, err)
			healthy = false
			// WHY NOT BREAK: Check all providers, report all failures
		}
	}

	if healthy {
		// WHY 200: Service is ready to handle requests
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	} else {
		// WHY 503: Service Unavailable - don't send traffic here
		// Kubernetes will stop routing requests to this pod
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
	}
}

// =============================================================================
// MAIN - APPLICATION ENTRY POINT
// =============================================================================
func main() {
	// Initialize controller with all providers configured
	controller := NewController()

	// Set up HTTP router
	// WHY GORILLA MUX: Better than default http.ServeMux
	// - Supports URL parameters like {id}
	// - Supports HTTP method filtering (.Methods("GET"))
	// - More features for REST APIs
	r := mux.NewRouter()


	r.HandleFunc("/resources", controller.HandleCreateResource).Methods("POST") // create 
	r.HandleFunc("/resources/{id}", controller.HandleGetResource).Methods("GET") // read
	r.HandleFunc("/resources/{id}", controller.HandleDeleteResource).Methods("DELETE") // dete
	r.HandleFunc("/health", controller.HandleHealthCheck).Methods("GET") // health check


	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" 
	}
	log.Printf("Controller listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
