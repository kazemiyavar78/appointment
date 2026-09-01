package pages

import (
	"net/url"
	"strconv"
	"strings"
)

// uintToString formats a uint for HTML option values in doctor_list.templ.
// Input: id. Output: decimal string.
func uintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// intToString formats an int for pagination and query values.
// Input: n. Output: decimal string.
func intToString(n int) string {
	return strconv.Itoa(n)
}

// int64ToString formats an int64 for HTML data attributes (e.g. unix timestamps).
// Input: n. Output: decimal string.
func int64ToString(n int64) string {
	return strconv.FormatInt(n, 10)
}

// doctorListHasFilters reports whether any list filter is active.
// Input: DoctorListView filter fields. Output: true when at least one filter is set.
func doctorListHasFilters(view DoctorListView) bool {
	return strings.TrimSpace(view.Query) != "" ||
		strings.TrimSpace(view.Date) != "" ||
		view.SpecialtyID > 0 ||
		view.ClinicID > 0
}

// doctorListPageURL builds a /doctors URL preserving filters for the given page.
// Input: view (current filters + FormAction), page number.
// Output: relative URL with query string.
func doctorListPageURL(view DoctorListView, page int) string {
	q := url.Values{}
	if s := strings.TrimSpace(view.Query); s != "" {
		q.Set("q", s)
	}
	if s := strings.TrimSpace(view.Date); s != "" {
		q.Set("date", s)
	}
	if view.SpecialtyID > 0 {
		q.Set("specialty_id", uintToString(view.SpecialtyID))
	}
	if view.ClinicID > 0 {
		q.Set("clinic_id", uintToString(view.ClinicID))
	}
	if page > 1 {
		q.Set("page", intToString(page))
	}
	base := strings.TrimSpace(view.FormAction)
	if base == "" {
		base = "/doctors"
	}
	encoded := q.Encode()
	if encoded == "" {
		return base
	}
	return base + "?" + encoded
}

// DoctorListURL آدرس فهرست پزشکان با فیلترهای فعلی را برای بازگشت از رزرو می‌سازد.
func DoctorListURL(view DoctorListView, page int) string {
	return doctorListPageURL(view, page)
}

// doctorListSpecialtyName returns the specialty label for the active specialty filter.
// Input: view. Output: specialty name, or empty when unset/unknown.
func doctorListSpecialtyName(view DoctorListView) string {
	if view.SpecialtyID == 0 {
		return ""
	}
	for _, sp := range view.Specialties {
		if sp.ID == view.SpecialtyID {
			return sp.Name
		}
	}
	return ""
}

// doctorListSpecialtyDescription توضیح کامل تخصص فیلترشده را برمی‌گرداند.
// ورودی: view لیست پزشکان. خروجی: Description تخصص انتخاب‌شده یا رشته خالی.
func doctorListSpecialtyDescription(view DoctorListView) string {
	if view.SpecialtyID == 0 {
		return ""
	}
	for _, sp := range view.Specialties {
		if sp.ID == view.SpecialtyID {
			return strings.TrimSpace(sp.Description)
		}
	}
	return ""
}

// doctorListClinicName returns the clinic label for the active clinic filter.
// Input: view. Output: clinic name, or empty when unset/unknown.
func doctorListClinicName(view DoctorListView) string {
	if view.ClinicID == 0 {
		return ""
	}
	for _, cl := range view.Clinics {
		if cl.ID == view.ClinicID {
			return cl.Name
		}
	}
	return ""
}

// doctorListClearURL builds a filter URL with one field removed (and page reset).
// Input: view, clearKey in {q,date,specialty_id,clinic_id}.
// Output: relative URL without that filter.
func doctorListClearURL(view DoctorListView, clearKey string) string {
	clone := view
	switch clearKey {
	case "q":
		clone.Query = ""
	case "date":
		clone.Date = ""
	case "specialty_id":
		clone.SpecialtyID = 0
	case "clinic_id":
		clone.ClinicID = 0
	}
	return doctorListPageURL(clone, 1)
}

// doctorListPaginationPages returns page numbers to show (with 0 as ellipsis).
// Input: current page and total pages. Output: compact page list for the nav.
func doctorListPaginationPages(page, totalPages int) []int {
	if totalPages <= 1 {
		return nil
	}
	if totalPages <= 7 {
		out := make([]int, 0, totalPages)
		for p := 1; p <= totalPages; p++ {
			out = append(out, p)
		}
		return out
	}

	set := map[int]struct{}{1: {}, totalPages: {}, page: {}}
	for _, p := range []int{page - 1, page + 1} {
		if p >= 1 && p <= totalPages {
			set[p] = struct{}{}
		}
	}
	keys := make([]int, 0, len(set))
	for p := 1; p <= totalPages; p++ {
		if _, ok := set[p]; ok {
			keys = append(keys, p)
		}
	}
	out := make([]int, 0, len(keys)+2)
	prev := 0
	for _, p := range keys {
		if prev > 0 && p-prev > 1 {
			out = append(out, 0)
		}
		out = append(out, p)
		prev = p
	}
	return out
}
