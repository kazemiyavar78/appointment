package news

import (
	"errors"
	"strings"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
)

// ErrInvalidInput is returned when required news fields are missing or invalid.
var ErrInvalidInput = errors.New("invalid news input")

// ErrForbidden is returned when the caller may not access the news clinic.
var ErrForbidden = errors.New("news access forbidden")

// ErrNotFound is returned when a news row does not exist.
var ErrNotFound = errors.New("news not found")

// Item is a news article enriched with clinic display name for public/admin views.
type Item struct {
	models.News
	ClinicName string
}

// SaveInput holds fields for create/update.
type SaveInput struct {
	ClinicID    uint
	Title       string
	Excerpt     string
	CoverURL    string
	Body        string
	IsPublished bool
	PublishedAt time.Time
}

// Service manages public news listing and admin CRUD.
type Service struct {
	News    *repository.NewsRepo
	Clinics *repository.ClinicRepo
}

// NewService constructs a news Service.
// Inputs: newsRepo, clinicRepo (for name enrichment).
// Output: pointer to Service.
func NewService(newsRepo *repository.NewsRepo, clinicRepo *repository.ClinicRepo) *Service {
	return &Service{News: newsRepo, Clinics: clinicRepo}
}

// ListAdmin returns all news for the given clinic IDs with clinic names.
// Inputs: clinicIDs.
// Output: enriched items or error.
func (s *Service) ListAdmin(clinicIDs []uint) ([]Item, error) {
	rows, err := s.News.ListByClinicIDs(clinicIDs)
	if err != nil {
		return nil, err
	}
	return s.enrich(rows)
}

// ListPublished returns published news for clinics (home/list pages).
// Inputs: clinicIDs, limit (0 = all).
// Output: enriched items or error.
func (s *Service) ListPublished(clinicIDs []uint, limit int) ([]Item, error) {
	rows, err := s.News.ListPublishedByClinicIDs(clinicIDs, limit)
	if err != nil {
		return nil, err
	}
	return s.enrich(rows)
}

// GetPublished returns one published news if it belongs to allowedClinicIDs.
// Inputs: id, allowedClinicIDs.
// Output: item or ErrNotFound / ErrForbidden.
func (s *Service) GetPublished(id uint, allowedClinicIDs []uint) (*Item, error) {
	row, err := s.News.GetByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if !row.IsPublished {
		return nil, ErrNotFound
	}
	if !containsID(allowedClinicIDs, row.ClinicID) {
		return nil, ErrForbidden
	}
	items, err := s.enrich([]models.News{*row})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// GetForAdmin returns a news row if its clinic is in allowedClinicIDs.
// Inputs: id, allowedClinicIDs.
// Output: news or ErrNotFound / ErrForbidden.
func (s *Service) GetForAdmin(id uint, allowedClinicIDs []uint) (*models.News, error) {
	row, err := s.News.GetByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if !containsID(allowedClinicIDs, row.ClinicID) {
		return nil, ErrForbidden
	}
	return row, nil
}

// Create validates, sanitizes, and inserts a news article.
// Inputs: in (save fields), allowedClinicIDs.
// Output: created row or ErrInvalidInput / ErrForbidden / DB error.
func (s *Service) Create(in SaveInput, allowedClinicIDs []uint) (*models.News, error) {
	row, err := s.buildRow(in, allowedClinicIDs)
	if err != nil {
		return nil, err
	}
	if err := s.News.Create(row); err != nil {
		return nil, err
	}
	return row, nil
}

// Update validates, sanitizes, and updates an existing news article.
// Inputs: id, in, allowedClinicIDs.
// Output: updated row or error.
func (s *Service) Update(id uint, in SaveInput, allowedClinicIDs []uint) (*models.News, error) {
	existing, err := s.GetForAdmin(id, allowedClinicIDs)
	if err != nil {
		return nil, err
	}
	row, err := s.buildRow(in, allowedClinicIDs)
	if err != nil {
		return nil, err
	}
	row.ID = existing.ID
	row.CreatedAt = existing.CreatedAt
	if err := s.News.Update(row); err != nil {
		return nil, err
	}
	return row, nil
}

// Delete removes a news article if accessible.
// Inputs: id, allowedClinicIDs.
// Output: error.
func (s *Service) Delete(id uint, allowedClinicIDs []uint) error {
	if _, err := s.GetForAdmin(id, allowedClinicIDs); err != nil {
		return err
	}
	return s.News.Delete(id)
}

func (s *Service) buildRow(in SaveInput, allowedClinicIDs []uint) (*models.News, error) {
	if !containsID(allowedClinicIDs, in.ClinicID) {
		return nil, ErrForbidden
	}
	title := SanitizeHTML(in.Title)
	excerpt := SanitizeHTML(in.Excerpt)
	body := SanitizeHTML(in.Body)
	if strings.TrimSpace(reStripTag.ReplaceAllString(title, "")) == "" {
		return nil, ErrInvalidInput
	}
	if PlainTextLength(excerpt) > 120 {
		return nil, ErrInvalidInput
	}
	publishedAt := in.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = time.Now()
	}
	return &models.News{
		ClinicID:    in.ClinicID,
		Title:       title,
		Excerpt:     excerpt,
		CoverURL:    strings.TrimSpace(in.CoverURL),
		Body:        body,
		PublishedAt: publishedAt,
		IsPublished: in.IsPublished,
	}, nil
}

func (s *Service) enrich(rows []models.News) ([]Item, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	nameByID := map[uint]string{}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		name, ok := nameByID[row.ClinicID]
		if !ok && s.Clinics != nil {
			if c, err := s.Clinics.GetByID(row.ClinicID); err == nil && c != nil {
				name = c.Name
			}
			nameByID[row.ClinicID] = name
		}
		out = append(out, Item{News: row, ClinicName: name})
	}
	return out, nil
}

func containsID(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
