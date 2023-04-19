package binary

type Metadata struct {
	Version   string `json:"version"`
	Digest    string `json:"TestContentDigest"`
	Signature string `json:"TestContentSignature"`
}
