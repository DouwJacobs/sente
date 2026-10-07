package statements

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"strings"

	"finance-tracker/internal/problem"
)

func Expand(files []InputFile) ([]InputFile, error) {
	result := []InputFile{}
	var total int64
	for _, f := range files {
		if strings.EqualFold(path.Ext(f.Name), ".zip") {
			z, err := zip.NewReader(bytes.NewReader(f.Content), int64(len(f.Content)))
			if err != nil {
				return nil, problem.New(400, "Invalid ZIP archive")
			}
			if len(z.File) > 100 {
				return nil, problem.New(400, "ZIP contains too many entries")
			}
			for _, entry := range z.File {
				if entry.FileInfo().IsDir() {
					continue
				}
				name := entry.Name
				if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || entry.Mode()&os.ModeSymlink != 0 {
					return nil, problem.New(400, "ZIP contains an unsafe entry")
				}
				ext := strings.ToLower(path.Ext(name))
				if ext != ".csv" && ext != ".ofx" {
					continue
				}
				if entry.UncompressedSize64 > 100<<20 {
					return nil, problem.New(400, "ZIP expansion exceeds 100 MiB")
				}
				rc, err := entry.Open()
				if err != nil {
					return nil, err
				}
				b, err := io.ReadAll(io.LimitReader(rc, (100<<20)-total+1))
				rc.Close()
				if err != nil {
					return nil, problem.New(400, "Could not read ZIP entry")
				}
				total += int64(len(b))
				if total > 100<<20 {
					return nil, problem.New(400, "ZIP expansion exceeds 100 MiB")
				}
				result = append(result, InputFile{path.Base(name), b})
			}
		} else {
			ext := strings.ToLower(path.Ext(f.Name))
			if ext != ".csv" && ext != ".ofx" {
				return nil, problem.New(400, "Upload FNB CSV, OFX, or ZIP files")
			}
			total += int64(len(f.Content))
			result = append(result, f)
		}
		if len(result) > 20 {
			return nil, problem.New(400, "Upload at most 20 CSV/OFX files per batch")
		}
	}
	if len(result) == 0 {
		return nil, problem.New(400, "No CSV or OFX files found")
	}
	return result, nil
}
