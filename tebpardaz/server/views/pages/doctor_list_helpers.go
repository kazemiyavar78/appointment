package pages

import "strconv"

// uintToString formats a uint for HTML option values in doctor_list.templ.
func uintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
