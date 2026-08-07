package websocket

import "net/http"

// AuthenticateClinic validates clinic client credentials on WebSocket upgrade.
// Prefer Handlers.authenticate which resolves against ClinicRepo.
// Inputs: HTTP upgrade request.
// Output: clinic key string and error.
func AuthenticateClinic(r *http.Request) (clinicKey string, err error) {
	key := r.URL.Query().Get("clinic_key")
	if key == "" {
		key = r.Header.Get("X-Clinic-Key")
	}
	if key == "" {
		return "", http.ErrNoCookie
	}
	return key, nil
}
