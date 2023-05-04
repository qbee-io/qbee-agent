package binary

type Metadata struct {
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	Signature string `json:"signature"`
}
