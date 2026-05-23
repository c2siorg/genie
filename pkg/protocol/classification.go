package protocol

// Classification labels message sensitivity. Governance policies key off this
// label to decide whether a particular sender or destination may see the
// message.
//
// Classifications are intentionally coarse to keep policy reasoning simple.
type Classification string

const (
	ClassPublic   Classification = "public"
	ClassInternal Classification = "internal"
	ClassPII      Classification = "pii"
	ClassSecret   Classification = "secret"
)

// MetaKeyClassification is the metadata key under which the classification
// label is stored on Message.Metadata.
const MetaKeyClassification = "classification"

// MetaKeyUserRoles is the metadata key under which authenticated user roles
// are stored when a request enters the system via the HTTP API.
const MetaKeyUserRoles = "user_roles"

// MetaKeyUserID stores the authenticated subject's user id.
const MetaKeyUserID = "user_id"

// ClassificationOf returns the classification stored in metadata, or the
// fallback if unset/blank.
func ClassificationOf(metadata map[string]any, fallback Classification) Classification {
	if metadata == nil {
		return fallback
	}
	if v, ok := metadata[MetaKeyClassification].(string); ok && v != "" {
		return Classification(v)
	}
	return fallback
}
