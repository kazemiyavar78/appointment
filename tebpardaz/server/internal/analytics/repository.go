package analytics

import (
	"context"
	"errors"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormVisitRepository persists buffered visits with GORM on appointment_tapesh.
type GormVisitRepository struct {
	DB *gorm.DB
}

// NewGormVisitRepository constructs a VisitRepository over the appointment database.
// Inputs: db (appointment GORM connection).
// Output: pointer to GormVisitRepository.
func NewGormVisitRepository(db *gorm.DB) *GormVisitRepository {
	return &GormVisitRepository{DB: db}
}

type ipBucket struct {
	count   uint
	lastAt  time.Time
	details []models.VisitDetail
}

// PersistVisits upserts visitor_ips (increment VisitCount) and inserts visit details.
// Inputs: ctx for cancellation, visits drained from the cache.
// Output: database error, if any; empty input is a no-op.
func (r *GormVisitRepository) PersistVisits(ctx context.Context, visits []VisitInfo) error {
	if r == nil || r.DB == nil || len(visits) == 0 {
		return nil
	}

	buckets := groupVisitsByIP(visits)
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for ip, bucket := range buckets {
			vip, err := upsertVisitorIP(tx, ip, bucket.count, bucket.lastAt)
			if err != nil {
				return err
			}
			if len(bucket.details) == 0 {
				continue
			}
			for i := range bucket.details {
				bucket.details[i].VisitorIPID = vip.ID
			}
			if err := tx.Create(&bucket.details).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// groupVisitsByIP aggregates buffered visits so each IP is upserted once per flush.
func groupVisitsByIP(visits []VisitInfo) map[string]*ipBucket {
	out := make(map[string]*ipBucket, len(visits))
	for _, v := range visits {
		ip := v.IPAddress
		if ip == "" {
			continue
		}
		b, ok := out[ip]
		if !ok {
			b = &ipBucket{}
			out[ip] = b
		}
		b.count++
		if v.VisitedAt.After(b.lastAt) {
			b.lastAt = v.VisitedAt
		}
		b.details = append(b.details, visitInfoToDetail(v))
	}
	return out
}

// visitInfoToDetail maps a buffered visit onto the VisitDetail model (VisitorIPID filled later).
func visitInfoToDetail(v VisitInfo) models.VisitDetail {
	created := v.VisitedAt
	if created.IsZero() {
		created = time.Now()
	}
	return models.VisitDetail{
		Host:         v.Host,
		ClinicID:     cloneClinicID(v.ClinicID),
		Path:         v.Path,
		FullURL:      v.FullURL,
		Browser:      v.Browser,
		OS:           v.OS,
		Referrer:     v.Referrer,
		IsFromGoogle: v.IsFromGoogle,
		CreatedAt:    created,
	}
}

// upsertVisitorIP inserts a new IP row or increments VisitCount and LastVisitAt.
func upsertVisitorIP(tx *gorm.DB, ip string, addCount uint, lastAt time.Time) (models.VisitorIP, error) {
	var vip models.VisitorIP
	err := tx.Where("ip_address = ?", ip).First(&vip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if lastAt.IsZero() {
			lastAt = time.Now()
		}
		vip = models.VisitorIP{
			IPAddress:   ip,
			VisitCount:  addCount,
			LastVisitAt: lastAt,
			CreatedAt:   lastAt,
		}
		if err := tx.Omit(clause.Associations).Create(&vip).Error; err != nil {
			return models.VisitorIP{}, err
		}
		return vip, nil
	}
	if err != nil {
		return models.VisitorIP{}, err
	}
	vip.VisitCount += addCount
	if lastAt.After(vip.LastVisitAt) {
		vip.LastVisitAt = lastAt
	}
	if err := tx.Model(&vip).Updates(map[string]any{
		"visit_count":   vip.VisitCount,
		"last_visit_at": vip.LastVisitAt,
	}).Error; err != nil {
		return models.VisitorIP{}, err
	}
	return vip, nil
}
