package constants

// TenantType matches Clinic.TenantType and drives public layout selection.
type TenantType string

const (
	// TenantPrivateOwnDomain is a private clinic on its own custom domain.
	TenantPrivateOwnDomain TenantType = "private_own_domain"
	// TenantPrivateTebpardaz is a private clinic on the platform base domain (slug/subdomain).
	TenantPrivateTebpardaz TenantType = "private_tebpardaz"
	// TenantOrganSubsidiary is a clinic listed under an organization site.
	TenantOrganSubsidiary TenantType = "organ_subsidiary"
)

// LayoutKind selects which templ layout the HTTP middleware should use.
type LayoutKind string

const (
	// LayoutPlatform is the company multi-clinic site (same UX as organ).
	LayoutPlatform LayoutKind = "platform"
	// LayoutOrgan is an organization multi-clinic site.
	LayoutOrgan LayoutKind = "organ"
	// LayoutPrivate is a single private clinic site.
	LayoutPrivate LayoutKind = "private"
)

// ClinicType classifies how a medical center is presented on the public site.
type ClinicType string

const (
	// ClinicTypeOrgan is an organization (multi-clinic) tenant.
	ClinicTypeOrgan ClinicType = "organ"
	// ClinicTypePrivate is a single private clinic tenant.
	ClinicTypePrivate ClinicType = "private"
)

// AppointmentStatus is the lifecycle state of a patient appointment.
type AppointmentStatus string

const (
	// AppointmentStatusPending awaits clinic confirmation.
	AppointmentStatusPending AppointmentStatus = "pending"
	// AppointmentStatusConfirmed is booked locally at the clinic.
	AppointmentStatusConfirmed AppointmentStatus = "confirmed"
	// AppointmentStatusCancelled was cancelled by patient or staff.
	AppointmentStatusCancelled AppointmentStatus = "cancelled"
	// AppointmentStatusFailed could not be saved on the clinic client.
	AppointmentStatusFailed AppointmentStatus = "failed"
)

// ApprovalStatus is the admin review state for synced doctors.
type ApprovalStatus string

const (
	// ApprovalPending awaits admin review.
	ApprovalPending ApprovalStatus = "pending"
	// ApprovalApproved is visible on the public site.
	ApprovalApproved ApprovalStatus = "approved"
	// ApprovalRejected was rejected by admin.
	ApprovalRejected ApprovalStatus = "rejected"
)

// UserRole is used by admin RBAC.
type UserRole string

const (
	// UserRoleSuperAdmin is a platform-wide administrator (no clinic scope).
	UserRoleSuperAdmin UserRole = "superadmin"
	// UserRoleAdmin manages a single assigned clinic back-office.
	UserRoleAdmin UserRole = "admin"
	// UserRoleOrganAdmin manages clinics under an assigned organization.
	UserRoleOrganAdmin UserRole = "organ_admin"
	// UserRoleEditor can manage news and approvals.
	UserRoleEditor UserRole = "editor"
	// UserRoleClinicAdmin is an alias for clinic-scoped admin.
	UserRoleClinicAdmin UserRole = "clinic_admin"
)
