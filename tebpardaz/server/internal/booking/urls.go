package booking

import (
	"fmt"
	"strings"

	"tebpardaz/shared/constants"
)

// BuildBookingURL returns the public booking path for the current layout.
// Inputs: layout kind, clinic slug (required for organ/platform), doctor slug.
// Output:
//   - private:  /booking/{doctor}
//   - organ/platform: /booking/{clinic}/{doctor}
func BuildBookingURL(layout constants.LayoutKind, clinicSlug, doctorSlug string) string {
	doctorSlug = strings.Trim(doctorSlug, "/")
	clinicSlug = strings.Trim(clinicSlug, "/")
	fmt.Println("layout", layout, clinicSlug, doctorSlug)
	if doctorSlug == "" {
		return "/doctors"
	}
	switch layout {
	case constants.LayoutPrivate:
		return "/booking/" + doctorSlug
	case constants.LayoutOrgan, constants.LayoutPlatform:
		if clinicSlug == "" {
			return "/doctors"
		}
		return "/booking/" + clinicSlug + "/" + doctorSlug
	default:
		return "/doctors"
	}
}
