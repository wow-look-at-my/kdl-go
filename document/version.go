package document

// Version represents a KDL specification version
type Version int

const (
	// VersionUnspecified indicates no version was specified; auto-detection will be used
	VersionUnspecified Version = 0
	// VersionV1 indicates KDL v1 specification
	VersionV1 Version = 1
	// VersionV2 indicates KDL v2 specification
	VersionV2 Version = 2
)
