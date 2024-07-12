package veracity

const (
	// LeafTypePlain is used for committing to plain values.
	LeafTypePlain         = uint8(0)
	PublicAssetsPrefix    = "publicassets/"
	ProtectedAssetsPrefix = "assets/"

	// To create smooth UX for basic or first-time users, we default to the verifiabledata proxy
	// on production. This gives us compact runes to verify inclusion of a List Events response.
	DefaultRemoteMassifURL = "https://app.datatrails.ai/verifiabledata/"
)
