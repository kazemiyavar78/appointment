package lis

// Client talks to the local Laboratory Information System for test results.
// TODO: define HTTP/SOAP transport based on clinic LIS vendor.
type Client struct {
	// TODO: BaseURL string
	// TODO: HTTPClient *http.Client
}

// NewClient constructs a LIS Client.
func NewClient(_ string) *Client {
	return &Client{}
}

// FetchResult looks up a lab result by national ID and/or barcode.
// TODO: call LIS API and map response to shared protocol DTOs.
func (c *Client) FetchResult(_ string, _ string) error {
	return nil
}
