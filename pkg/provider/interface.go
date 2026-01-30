package provider

import (
	"context"

	"github.com/Zhichengu1/mock-control-plane/pkg/models"
)

// =============================================================================
// VENDOR PROVIDER INTERFACE
// =============================================================================
// This interface defines the contract that all vendor integrations must
// implement. It follows the Strategy Pattern, allowing the controller to
// work with different vendors (Sony, AWS, etc.) through a common interface.
//
// Design Principles:
// 1. Vendor Agnostic: The controller doesn't need to know vendor specifics
// 2. CRUD Operations: Standard Create, Read, Update, Delete pattern
// 3. Context Support: All methods accept context for timeout/cancellation
// 4. Error Handling: Errors bubble up for controller-level handling
//
// Implementation Requirements:
// - Each provider must handle its own authentication
// - Providers transform ForgeResource ↔ Vendor-specific formats
// - Providers must be safe for concurrent use
// - Providers should implement retry logic for transient failures
//
// Example Usage:
//
//	var provider VendorProvider = NewSonyProvider(baseURL, apiKey)
//	status, err := provider.Create(ctx, &resource)
//
// =============================================================================

// VendorProvider defines the contract all vendor integrations must implement.
// This is the blueprint that all vendor-specific controllers follow.
//
// The interface abstracts vendor differences, enabling:
// - Uniform resource lifecycle management
// - Easy addition of new vendors
// - Simplified testing with mock implementations
type VendorProvider interface {
	// Create provisions a new resource in the vendor system.
	//
	// This method transforms the ForgeResource's Spec (desired state) into
	// a vendor-specific API call and returns the observed state.
	//
	// Flow:
	// 1. Transform ForgeResource → Vendor Request
	// 2. Execute vendor API call (POST)
	// 3. Transform Vendor Response → ResourceStatus
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout. Implementations should
	//          respect context cancellation and return early if cancelled.
	//   - resource: The ForgeResource containing desired state. The Spec field
	//               contains all configuration needed for provisioning.
	//
	// Returns:
	//   - *ResourceStatus: The observed state after creation attempt.
	//                      Phase will be "Running" on success, "Failed" on error.
	//                      VendorID contains the vendor's unique identifier.
	//   - error: Non-nil if the operation failed. May be transient (retry)
	//            or permanent (configuration error).
	//
	// Example:
	//   status, err := provider.Create(ctx, &models.ForgeResource{
	//       Name: "my-camera",
	//       Spec: models.ResourceSpec{VendorType: "sony", Resolution: "4K"},
	//   })
	Create(ctx context.Context, resource *models.ForgeResource) (*models.ResourceStatus, error)

	// Read retrieves the current state from the vendor system.
	//
	// This method queries the vendor API to get the latest status of a
	// previously created resource. Used for:
	// - Status synchronization
	// - Health checking
	// - Drift detection (comparing Spec vs actual state)
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - vendorID: The vendor's unique identifier for the resource.
	//               This was returned in ResourceStatus.VendorID from Create().
	//
	// Returns:
	//   - *ResourceStatus: Current observed state from the vendor.
	//                      All status fields reflect real-time data.
	//   - error: Non-nil if the resource doesn't exist or API call failed.
	//            404 Not Found should return a status with Phase="Failed".
	//
	// Example:
	//   status, err := provider.Read(ctx, "sony-device-12345")
	Read(ctx context.Context, vendorID string) (*models.ResourceStatus, error)

	// Update modifies an existing resource in the vendor system.
	//
	// This method applies changes from an updated ForgeResource Spec to
	// the actual vendor resource. Only changed fields should be updated.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - resource: The ForgeResource with updated Spec.
	//               Status.VendorID must be set to identify the resource.
	//
	// Returns:
	//   - *ResourceStatus: State after the update attempt.
	//                      Phase will be "Running" or "Updating" on success.
	//   - error: Non-nil if update failed. Common causes:
	//            - Resource doesn't exist (was deleted externally)
	//            - Invalid configuration in Spec
	//            - Vendor API error
	//
	// Example:
	//   resource.Spec.Resolution = "1080p"
	//   status, err := provider.Update(ctx, resource)
	Update(ctx context.Context, resource *models.ForgeResource) (*models.ResourceStatus, error)

	// Delete removes the resource from the vendor system.
	//
	// This method is idempotent - calling Delete on an already-deleted
	// resource should not return an error. This simplifies cleanup logic.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - vendorID: The vendor's unique identifier for the resource
	//
	// Returns:
	//   - error: Non-nil only for actual failures (not 404/already deleted).
	//            Common causes:
	//            - Authentication failure
	//            - Network error
	//            - Resource in use / cannot be deleted
	//
	// Example:
	//   err := provider.Delete(ctx, "sony-device-12345")
	Delete(ctx context.Context, vendorID string) error

	// HealthCheck verifies vendor API connectivity.
	//
	// This method tests whether the vendor API is reachable and responsive.
	// It does NOT check individual resource health (use Read for that).
	//
	// Use cases:
	// - Startup validation: Ensure vendor is reachable before accepting requests
	// - Monitoring: Periodic checks to detect vendor outages
	// - Circuit breaker: Disable vendor operations during outages
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout.
	//          Should use a short timeout (5-10 seconds) for health checks.
	//
	// Returns:
	//   - error: nil if vendor is healthy and reachable.
	//            Non-nil with details if health check failed:
	//            - Connection refused: Vendor API is down
	//            - Timeout: Vendor API is slow/overloaded
	//            - 401/403: Authentication issue
	//            - 5xx: Vendor internal error
	//
	// Example:
	//   if err := provider.HealthCheck(ctx); err != nil {
	//       log.Warn("Vendor unhealthy", "vendor", "sony", "error", err)
	//   }
	HealthCheck(ctx context.Context) error
}
