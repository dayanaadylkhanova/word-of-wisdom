package entity

type Challenge struct {
	Version    int    `json:"version"`
	Algo       string `json:"algo"`
	Difficulty int    `json:"difficulty"`
	SaltB64    string `json:"salt_b64"`
	Expires    int64  `json:"expires"`
}

type Solution struct {
	Nonce string `json:"nonce"`
}
