package models

// =============================================================================
// SONY VENDOR MODELS
// =============================================================================
// These structures define the data formats for communicating with Sony's
// device management API. They handle the transformation between Forge's
// internal representation and Sony's expected API format.
//
// Sony API Integration Flow:
// 1. ForgeResource → SonyDeviceRequest (outbound transformation)
// 2. HTTP POST/PUT to Sony API
// 3. SonyDeviceResponse → ResourceStatus (inbound transformation)
// =============================================================================

// SonyDeviceRequest represents the payload Sony's API expects for device
// provisioning and management operations. This struct is built from a
// ForgeResource by the SonyProvider before sending to the vendor API.
//
// Example JSON sent to Sony API:
//
//	{
//	  "device_name": "stadium-cam-1",
//	  "model": "HDC-5500",
//	  "settings": {"resolution": "4K", "frame_rate": "59.94"},
//	  "ip_address": "10.0.1.50",
//	  "stream_config": {...}
//	}
type SonyDeviceRequest struct {
	// DeviceName is the human-readable name for the device.
	// Maps from ForgeResource.Name
	DeviceName string `json:"device_name"`

	// Model specifies the Sony device model.
	// Common values: "HDC-5500", "HDC-3500", "HDC-P50", "PXW-Z750"
	// Extracted from ForgeResource.Spec.Config["sony_model"]
	Model string `json:"model"`

	// Settings contains device configuration parameters.
	// Keys are Sony-specific setting names.
	// Built from ForgeResource.Spec fields.
	Settings map[string]string `json:"settings"`

	// =========================================================================
	// SONY-SPECIFIC FIELDS (from Sony API documentation)
	// =========================================================================

	// IPAddress is the network address for the device.
	// Required for networked devices; used for direct communication.
	// Format: IPv4 ("192.168.1.100") or IPv6
	IPAddress string `json:"ip_address,omitempty"`

	// Port is the network port for device communication.
	// Default varies by protocol (e.g., 80 for HTTP, 554 for RTSP)
	Port int `json:"port,omitempty"`

	// StreamConfig contains output streaming configuration.
	// Used when the device needs to output video to a destination.
	StreamConfig *SonyStreamConfig `json:"stream_config,omitempty"`

	// RecordingConfig contains recording settings.
	// Used when the device should record content locally or to storage.
	RecordingConfig *SonyRecordingConfig `json:"recording_config,omitempty"`

	// NetworkConfig specifies network-related settings.
	// Includes VLAN, bonding, and failover configuration.
	NetworkConfig *SonyNetworkConfig `json:"network_config,omitempty"`

	// TallyConfig configures tally light behavior.
	// Tally lights indicate on-air status to camera operators.
	TallyConfig *SonyTallyConfig `json:"tally_config,omitempty"`

	// Metadata contains optional key-value pairs for tracking.
	// Useful for tagging devices with customer or project info.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SonyStreamConfig defines streaming output settings for Sony devices.
// This configures how the device delivers video to destinations.
type SonyStreamConfig struct {
	// Enabled indicates if streaming output is active.
	Enabled bool `json:"enabled"`

	// Protocol specifies the streaming protocol.
	// Supported: "RTMP", "SRT", "RTSP", "NDI", "SDI-over-IP"
	Protocol string `json:"protocol"`

	// DestinationURL is where the stream should be sent.
	// Format depends on protocol (e.g., rtmp://host/app/key)
	DestinationURL string `json:"destination_url"`

	// Resolution is the output resolution.
	// Sony format: "3840x2160", "1920x1080", "1280x720"
	Resolution string `json:"resolution"`

	// Bitrate is the encoding bitrate in kbps (note: kbps, not bps).
	// Sony uses kilobits, so divide ForgeResource.Spec.Bitrate by 1000.
	Bitrate int `json:"bitrate"`

	// FrameRate is frames per second (e.g., 59.94, 29.97, 25).
	FrameRate float64 `json:"frame_rate"`

	// Codec is the encoding codec.
	// Sony supported: "H.264", "H.265", "XAVC"
	Codec string `json:"codec"`

	// LatencyMode sets the encoding latency.
	// Sony values: "ultra_low", "low", "normal"
	LatencyMode string `json:"latency_mode"`

	// SRTPassphrase is the encryption passphrase for SRT streams.
	// Required when Protocol is "SRT" and encryption is enabled.
	SRTPassphrase string `json:"srt_passphrase,omitempty"`

	// SRTLatency is the SRT latency in milliseconds.
	// Typical values: 120-250ms for low latency, 500-1000ms for reliability.
	SRTLatency int `json:"srt_latency,omitempty"`
}

// SonyRecordingConfig defines recording settings for Sony devices.
type SonyRecordingConfig struct {
	// Enabled indicates if recording is active.
	Enabled bool `json:"enabled"`

	// StoragePath is the recording destination.
	// Can be local path ("/media/recordings") or network share.
	StoragePath string `json:"storage_path"`

	// Format is the recording container format.
	// Sony supported: "MXF", "XAVC", "ProRes", "MP4"
	Format string `json:"format"`

	// Quality sets the recording quality preset.
	// Values: "proxy" (low), "production" (high), "master" (maximum)
	Quality string `json:"quality"`

	// MaxDurationMinutes limits individual recording file length.
	// New file is created when limit is reached (continuous recording).
	MaxDurationMinutes int `json:"max_duration_minutes,omitempty"`

	// RetentionDays specifies how long recordings are kept.
	// 0 means indefinite retention.
	RetentionDays int `json:"retention_days,omitempty"`
}

// SonyNetworkConfig defines network settings for Sony devices.
type SonyNetworkConfig struct {
	// PrimaryInterface is the main network interface.
	// Values: "eth0", "eth1", "bond0"
	PrimaryInterface string `json:"primary_interface"`

	// VLANID is the VLAN tag for network isolation.
	// 0 or empty means untagged traffic.
	VLANID int `json:"vlan_id,omitempty"`

	// BondingEnabled indicates if network interface bonding is active.
	// Provides redundancy and increased bandwidth.
	BondingEnabled bool `json:"bonding_enabled,omitempty"`

	// BondingMode specifies the bonding algorithm.
	// Values: "active-backup", "802.3ad" (LACP), "balance-rr"
	BondingMode string `json:"bonding_mode,omitempty"`

	// MTU is the Maximum Transmission Unit size.
	// Standard is 1500; jumbo frames use 9000.
	MTU int `json:"mtu,omitempty"`
}

// SonyTallyConfig defines tally light configuration.
type SonyTallyConfig struct {
	// Enabled indicates if tally control is active.
	Enabled bool `json:"enabled"`

	// Color sets the tally light color when active.
	// Values: "red" (on-air), "green" (preview), "yellow" (warning)
	Color string `json:"color"`

	// ControlProtocol specifies how tally is controlled.
	// Values: "TSL", "GPIO", "IP"
	ControlProtocol string `json:"control_protocol"`

	// ControlAddress is the address for tally control commands.
	// Format depends on protocol.
	ControlAddress string `json:"control_address,omitempty"`
}

// =============================================================================
// SONY API RESPONSE MODELS
// =============================================================================

// SonyDeviceResponse represents Sony's API response after device operations.
// This is returned by Sony's API and transformed into ResourceStatus.
type SonyDeviceResponse struct {
	// DeviceID is Sony's unique identifier for this device.
	// Store this in ResourceStatus.VendorID for future operations.
	DeviceID string `json:"device_id"`

	// Status indicates the device's current state.
	// Sony values: "active", "inactive", "error", "provisioning", "maintenance"
	Status string `json:"status"`

	// Message provides additional details about the status.
	// Contains error details when Status is "error".
	Message string `json:"message"`

	// =========================================================================
	// EXTENDED RESPONSE FIELDS
	// =========================================================================

	// Model is the device model as registered in Sony's system.
	Model string `json:"model,omitempty"`

	// FirmwareVersion is the current firmware version.
	FirmwareVersion string `json:"firmware_version,omitempty"`

	// SerialNumber is the device's hardware serial number.
	SerialNumber string `json:"serial_number,omitempty"`

	// IPAddress is the device's current network address.
	IPAddress string `json:"ip_address,omitempty"`

	// StreamStatus provides info about active streaming.
	StreamStatus *SonyStreamStatus `json:"stream_status,omitempty"`

	// HealthMetrics contains device health information.
	HealthMetrics *SonyHealthMetrics `json:"health_metrics,omitempty"`

	// CreatedAt is when the device was registered in Sony's system.
	CreatedAt string `json:"created_at,omitempty"`

	// UpdatedAt is when the device was last modified.
	UpdatedAt string `json:"updated_at,omitempty"`

	// ErrorCode is a machine-readable error code (when Status is "error").
	// Used for programmatic error handling.
	ErrorCode string `json:"error_code,omitempty"`

	// ErrorDetails provides structured error information.
	ErrorDetails *SonyErrorDetails `json:"error_details,omitempty"`
}

// SonyStreamStatus provides information about active streaming.
type SonyStreamStatus struct {
	// IsStreaming indicates if the device is actively streaming.
	IsStreaming bool `json:"is_streaming"`

	// CurrentBitrate is the actual measured bitrate in kbps.
	CurrentBitrate int `json:"current_bitrate,omitempty"`

	// DroppedFrames is the count of dropped frames since stream start.
	DroppedFrames int64 `json:"dropped_frames,omitempty"`

	// UptimeSeconds is how long the current stream has been running.
	UptimeSeconds int64 `json:"uptime_seconds,omitempty"`

	// ViewerCount is the number of active viewers (for multi-output).
	ViewerCount int `json:"viewer_count,omitempty"`

	// DestinationStatus tracks each output destination's status.
	DestinationStatus []SonyDestinationStatus `json:"destination_status,omitempty"`
}

// SonyDestinationStatus tracks status of individual stream destinations.
type SonyDestinationStatus struct {
	// URL is the destination being tracked.
	URL string `json:"url"`

	// Connected indicates if the destination is reachable.
	Connected bool `json:"connected"`

	// LastError is the most recent error for this destination.
	LastError string `json:"last_error,omitempty"`

	// BytesSent is the total bytes sent to this destination.
	BytesSent int64 `json:"bytes_sent,omitempty"`
}

// SonyHealthMetrics contains device health information.
type SonyHealthMetrics struct {
	// CPUUsagePercent is current CPU utilization.
	CPUUsagePercent float64 `json:"cpu_usage_percent"`

	// MemoryUsagePercent is current memory utilization.
	MemoryUsagePercent float64 `json:"memory_usage_percent"`

	// Temperature is the device temperature in Celsius.
	Temperature float64 `json:"temperature_celsius"`

	// FanSpeedRPM is the current fan speed.
	FanSpeedRPM int `json:"fan_speed_rpm,omitempty"`

	// StorageUsedGB is the used storage in gigabytes.
	StorageUsedGB float64 `json:"storage_used_gb,omitempty"`

	// StorageTotalGB is the total storage capacity.
	StorageTotalGB float64 `json:"storage_total_gb,omitempty"`

	// NetworkRxMbps is the network receive rate in Mbps.
	NetworkRxMbps float64 `json:"network_rx_mbps,omitempty"`

	// NetworkTxMbps is the network transmit rate in Mbps.
	NetworkTxMbps float64 `json:"network_tx_mbps,omitempty"`

	// LastChecked is when these metrics were collected.
	LastChecked string `json:"last_checked,omitempty"`
}

// SonyErrorDetails provides structured error information.
type SonyErrorDetails struct {
	// Code is the error code.
	Code string `json:"code"`

	// Category groups related errors.
	// Values: "network", "hardware", "configuration", "authentication"
	Category string `json:"category"`

	// Severity indicates error impact.
	// Values: "warning", "error", "critical"
	Severity string `json:"severity"`

	// Suggestion provides remediation guidance.
	Suggestion string `json:"suggestion,omitempty"`

	// DocumentationURL links to relevant documentation.
	DocumentationURL string `json:"documentation_url,omitempty"`
}

// =============================================================================
// AWS VENDOR MODELS (Future Implementation)
// =============================================================================
// These structures define the data formats for AWS MediaLive integration.
// AWS MediaLive is used for cloud-based video encoding and delivery.
// =============================================================================

// AWSResourceRequest defines the payload for creating AWS MediaLive resources.
// This maps to AWS MediaLive's CreateChannel API.
//
// Reference: https://docs.aws.amazon.com/medialive/latest/apireference/channels.html
type AWSResourceRequest struct {
	// ChannelName is the name for the MediaLive channel.
	// Maps from ForgeResource.Name
	ChannelName string `json:"channel_name"`

	// ChannelClass determines redundancy.
	// Values: "STANDARD" (single pipeline), "SINGLE_PIPELINE" (cost-effective)
	ChannelClass string `json:"channel_class"`

	// InputSpecification defines the expected input characteristics.
	InputSpecification AWSInputSpec `json:"input_specification"`

	// Destinations defines where the channel outputs video.
	Destinations []AWSDestination `json:"destinations"`

	// EncoderSettings contains all encoding configuration.
	EncoderSettings AWSEncoderSettings `json:"encoder_settings"`

	// Tags are AWS resource tags for organization and billing.
	Tags map[string]string `json:"tags,omitempty"`

	// RoleArn is the IAM role ARN for MediaLive to assume.
	RoleArn string `json:"role_arn,omitempty"`

	// LogLevel sets the logging verbosity.
	// Values: "DISABLED", "ERROR", "WARNING", "INFO", "DEBUG"
	LogLevel string `json:"log_level,omitempty"`
}

// AWSInputSpec defines the expected input characteristics.
type AWSInputSpec struct {
	// Codec is the input codec.
	// Values: "AVC", "HEVC", "MPEG2"
	Codec string `json:"codec"`

	// Resolution is the maximum input resolution.
	// Values: "SD", "HD", "UHD"
	Resolution string `json:"resolution"`

	// MaximumBitrate is the maximum input bitrate.
	// Values: "MAX_10_MBPS", "MAX_20_MBPS", "MAX_50_MBPS"
	MaximumBitrate string `json:"maximum_bitrate"`
}

// AWSDestination defines an output destination.
type AWSDestination struct {
	// ID is a unique identifier for this destination.
	ID string `json:"id"`

	// Settings contains the destination URLs.
	Settings []AWSDestinationSettings `json:"settings"`

	// MediaPackageSettings for MediaPackage destinations.
	MediaPackageSettings []AWSMediaPackageSettings `json:"media_package_settings,omitempty"`
}

// AWSDestinationSettings contains destination URL configuration.
type AWSDestinationSettings struct {
	// URL is the output destination URL.
	URL string `json:"url"`

	// Username for authentication (if required).
	Username string `json:"username,omitempty"`

	// PasswordParam is the Parameter Store parameter name for the password.
	PasswordParam string `json:"password_param,omitempty"`

	// StreamName is the stream key (for RTMP).
	StreamName string `json:"stream_name,omitempty"`
}

// AWSMediaPackageSettings for AWS MediaPackage integration.
type AWSMediaPackageSettings struct {
	// ChannelId is the MediaPackage channel ID.
	ChannelId string `json:"channel_id"`
}

// AWSEncoderSettings contains encoding configuration.
type AWSEncoderSettings struct {
	// VideoDescriptions defines video encoding settings.
	VideoDescriptions []AWSVideoDescription `json:"video_descriptions"`

	// AudioDescriptions defines audio encoding settings.
	AudioDescriptions []AWSAudioDescription `json:"audio_descriptions"`

	// OutputGroups defines how outputs are grouped and delivered.
	OutputGroups []AWSOutputGroup `json:"output_groups"`
}

// AWSVideoDescription defines a video encoding configuration.
type AWSVideoDescription struct {
	// Name is the identifier for this video description.
	Name string `json:"name"`

	// Width is the output width in pixels.
	Width int `json:"width"`

	// Height is the output height in pixels.
	Height int `json:"height"`

	// CodecSettings contains codec-specific settings.
	CodecSettings AWSVideoCodecSettings `json:"codec_settings"`
}

// AWSVideoCodecSettings contains video codec configuration.
type AWSVideoCodecSettings struct {
	// H264Settings for H.264 encoding.
	H264Settings *AWSH264Settings `json:"h264_settings,omitempty"`

	// H265Settings for H.265/HEVC encoding.
	H265Settings *AWSH265Settings `json:"h265_settings,omitempty"`
}

// AWSH264Settings contains H.264 encoding parameters.
type AWSH264Settings struct {
	// Bitrate is the output bitrate in bps.
	Bitrate int `json:"bitrate"`

	// FramerateDenominator for frame rate (e.g., 1001 for 29.97).
	FramerateDenominator int `json:"framerate_denominator"`

	// FramerateNumerator for frame rate (e.g., 30000 for 29.97).
	FramerateNumerator int `json:"framerate_numerator"`

	// Profile is the H.264 profile.
	// Values: "BASELINE", "MAIN", "HIGH"
	Profile string `json:"profile"`

	// Level is the H.264 level.
	// Values: "H264_LEVEL_AUTO", "H264_LEVEL_4_1", "H264_LEVEL_5_1"
	Level string `json:"level"`

	// RateControlMode controls bitrate variance.
	// Values: "CBR" (constant), "VBR" (variable), "QVBR" (quality-defined)
	RateControlMode string `json:"rate_control_mode"`
}

// AWSH265Settings contains H.265/HEVC encoding parameters.
type AWSH265Settings struct {
	// Bitrate is the output bitrate in bps.
	Bitrate int `json:"bitrate"`

	// FramerateDenominator for frame rate.
	FramerateDenominator int `json:"framerate_denominator"`

	// FramerateNumerator for frame rate.
	FramerateNumerator int `json:"framerate_numerator"`

	// Profile is the H.265 profile.
	// Values: "MAIN", "MAIN_10BIT"
	Profile string `json:"profile"`

	// Tier is the H.265 tier.
	// Values: "MAIN", "HIGH"
	Tier string `json:"tier"`

	// Level is the H.265 level.
	// Values: "H265_LEVEL_AUTO", "H265_LEVEL_5_1", "H265_LEVEL_6"
	Level string `json:"level"`
}

// AWSAudioDescription defines audio encoding configuration.
type AWSAudioDescription struct {
	// Name is the identifier for this audio description.
	Name string `json:"name"`

	// AudioSelectorName references the input audio selector.
	AudioSelectorName string `json:"audio_selector_name"`

	// CodecSettings contains audio codec configuration.
	CodecSettings AWSAudioCodecSettings `json:"codec_settings"`
}

// AWSAudioCodecSettings contains audio codec configuration.
type AWSAudioCodecSettings struct {
	// AacSettings for AAC audio encoding.
	AacSettings *AWSAacSettings `json:"aac_settings,omitempty"`
}

// AWSAacSettings contains AAC audio encoding parameters.
type AWSAacSettings struct {
	// Bitrate is the audio bitrate in bps.
	Bitrate float64 `json:"bitrate"`

	// SampleRate is the audio sample rate in Hz.
	SampleRate float64 `json:"sample_rate"`

	// CodingMode defines the channel configuration.
	// Values: "AD_RECEIVER_MIX", "CODING_MODE_1_0" (mono),
	//         "CODING_MODE_2_0" (stereo), "CODING_MODE_5_1"
	CodingMode string `json:"coding_mode"`
}

// AWSOutputGroup defines how outputs are packaged and delivered.
type AWSOutputGroup struct {
	// Name is the identifier for this output group.
	Name string `json:"name"`

	// OutputGroupSettings contains delivery-specific settings.
	OutputGroupSettings AWSOutputGroupSettings `json:"output_group_settings"`

	// Outputs lists individual outputs in this group.
	Outputs []AWSOutput `json:"outputs"`
}

// AWSOutputGroupSettings contains output group configuration.
type AWSOutputGroupSettings struct {
	// HlsGroupSettings for HLS output.
	HlsGroupSettings *AWSHlsGroupSettings `json:"hls_group_settings,omitempty"`

	// RtmpGroupSettings for RTMP output.
	RtmpGroupSettings *AWSRtmpGroupSettings `json:"rtmp_group_settings,omitempty"`

	// MediaPackageGroupSettings for MediaPackage output.
	MediaPackageGroupSettings *AWSMediaPackageGroupSettings `json:"media_package_group_settings,omitempty"`
}

// AWSHlsGroupSettings for HLS output configuration.
type AWSHlsGroupSettings struct {
	// Destination references a destination ID.
	Destination AWSDestinationRef `json:"destination"`

	// SegmentLength is the target segment duration in seconds.
	SegmentLength int `json:"segment_length"`

	// MinSegmentLength is the minimum segment duration.
	MinSegmentLength int `json:"min_segment_length,omitempty"`
}

// AWSRtmpGroupSettings for RTMP output configuration.
type AWSRtmpGroupSettings struct {
	// AuthenticationScheme for RTMP authentication.
	// Values: "AKAMAI", "COMMON"
	AuthenticationScheme string `json:"authentication_scheme"`
}

// AWSMediaPackageGroupSettings for MediaPackage output.
type AWSMediaPackageGroupSettings struct {
	// Destination references a destination ID.
	Destination AWSDestinationRef `json:"destination"`
}

// AWSDestinationRef references a destination by ID.
type AWSDestinationRef struct {
	// DestinationRefId is the destination ID to reference.
	DestinationRefId string `json:"destination_ref_id"`
}

// AWSOutput defines an individual output.
type AWSOutput struct {
	// OutputName is the name for this output.
	OutputName string `json:"output_name"`

	// VideoDescriptionName references a video description.
	VideoDescriptionName string `json:"video_description_name"`

	// AudioDescriptionNames references audio descriptions.
	AudioDescriptionNames []string `json:"audio_description_names"`

	// OutputSettings contains format-specific settings.
	OutputSettings AWSOutputSettings `json:"output_settings"`
}

// AWSOutputSettings contains output format settings.
type AWSOutputSettings struct {
	// HlsOutputSettings for HLS outputs.
	HlsOutputSettings *AWSHlsOutputSettings `json:"hls_output_settings,omitempty"`

	// RtmpOutputSettings for RTMP outputs.
	RtmpOutputSettings *AWSRtmpOutputSettings `json:"rtmp_output_settings,omitempty"`
}

// AWSHlsOutputSettings for HLS output.
type AWSHlsOutputSettings struct {
	// NameModifier appends to the base filename.
	NameModifier string `json:"name_modifier,omitempty"`

	// HlsSettings contains HLS-specific settings.
	HlsSettings AWSHlsSettings `json:"hls_settings"`
}

// AWSHlsSettings for HLS configuration.
type AWSHlsSettings struct {
	// StandardHlsSettings for standard HLS.
	StandardHlsSettings *AWSStandardHlsSettings `json:"standard_hls_settings,omitempty"`
}

// AWSStandardHlsSettings for standard HLS.
type AWSStandardHlsSettings struct {
	// M3u8Settings for M3U8 playlist configuration.
	M3u8Settings AWSM3u8Settings `json:"m3u8_settings"`
}

// AWSM3u8Settings for M3U8 playlist.
type AWSM3u8Settings struct {
	// AudioFramesPerPes sets audio frames per PES.
	AudioFramesPerPes int `json:"audio_frames_per_pes,omitempty"`
}

// AWSRtmpOutputSettings for RTMP output.
type AWSRtmpOutputSettings struct {
	// Destination references a destination.
	Destination AWSDestinationRef `json:"destination"`

	// NumRetries is the number of retry attempts.
	NumRetries int `json:"num_retries,omitempty"`
}

// AWSResourceResponse represents the response from AWS MediaLive operations.
type AWSResourceResponse struct {
	// ChannelId is the AWS-assigned channel ID.
	ChannelId string `json:"channel_id"`

	// Arn is the channel's Amazon Resource Name.
	Arn string `json:"arn"`

	// State is the channel state.
	// Values: "CREATING", "CREATE_FAILED", "IDLE", "STARTING",
	//         "RUNNING", "RECOVERING", "STOPPING", "DELETING", "DELETED"
	State string `json:"state"`

	// Name is the channel name.
	Name string `json:"name,omitempty"`

	// PipelinesRunningCount shows how many pipelines are active.
	PipelinesRunningCount int `json:"pipelines_running_count,omitempty"`

	// ErrorMessage contains error details if State is "CREATE_FAILED".
	ErrorMessage string `json:"error_message,omitempty"`

	// EgressEndpoints lists the output endpoints.
	EgressEndpoints []AWSEgressEndpoint `json:"egress_endpoints,omitempty"`
}

// AWSEgressEndpoint represents an output endpoint.
type AWSEgressEndpoint struct {
	// SourceIp is the source IP for this endpoint.
	SourceIp string `json:"source_ip"`
}
