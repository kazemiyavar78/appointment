package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Store فایل‌های پیوست بیمار و تصویر تبلیغ را زیر یک ریشه مشترک می‌خواند.
type Store struct {
	root string
}

// Open ریشه ذخیره‌سازی را اعتبارسنجی می‌کند.
// Inputs: root مسیر مطلق FILE_STORAGE_ROOT. Output: Store یا خطا اگر مسیر خالی باشد.
func Open(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("file storage root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Store{root: abs}, nil
}

// Root مسیر مطلق ریشه را برمی‌گرداند.
// Inputs: none. Output: absolute root directory.
func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Absolute مسیر نسبی ذخیره‌شده در دیتابیس را به مسیر مطلق زیر ریشه تبدیل می‌کند.
// Inputs: relPath مانند category/YYYY-MM/clinicCode/file. Output: مسیر مطلق یا خطا اگر خارج از ریشه باشد.
func (s *Store) Absolute(relPath string) (string, error) {
	if s == nil || s.root == "" {
		return "", errors.New("file storage is not configured")
	}
	rel := strings.TrimSpace(strings.ReplaceAll(relPath, "\\", "/"))
	rel = strings.Trim(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", errors.New("invalid relative path")
	}
	abs := filepath.Clean(filepath.Join(s.root, filepath.FromSlash(rel)))
	root := filepath.Clean(s.root)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", errors.New("path escapes storage root")
	}
	return abs, nil
}

// AdImage مسیر یک تصویر تبلیغ را فقط از روی نام فایل (بدون پوشه) حل می‌کند.
// Inputs: name نام فایل داخل پوشه ads. Output: مسیر مطلق یا خطا.
func (s *Store) AdImage(name string) (string, error) {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == "" || base == ".." || base != strings.TrimSpace(name) {
		return "", errors.New("invalid ad image name")
	}
	return s.Absolute("ads/" + base)
}
