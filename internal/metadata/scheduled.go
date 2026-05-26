package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ScheduledJob struct {
	Name      string            `yaml:"name"`
	Title     string            `yaml:"title"`
	Titles    map[string]string `yaml:"titles"`
	Schedule  string            `yaml:"schedule"`
	Processor string            `yaml:"processor"`
	Params    map[string]any    `yaml:"params"`
	Enabled   bool              `yaml:"enabled"`
	OnError   string            `yaml:"on_error"`
	Timeout   int               `yaml:"timeout"` // seconds
}

// DisplayName возвращает заголовок регламентного задания с учётом языка.
func (j *ScheduledJob) DisplayName(lang string) string {
	if lang != "" {
		if v, ok := j.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if j.Title != "" {
		return j.Title
	}
	return j.Name
}

func LoadScheduledFile(path string) (*ScheduledJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scheduled: read %s: %w", path, err)
	}
	var job ScheduledJob
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("scheduled: parse %s: %w", path, err)
	}
	if job.OnError == "" {
		job.OnError = "continue"
	}
	if job.Timeout == 0 {
		job.Timeout = 3600
	}
	return &job, nil
}

func LoadScheduledDir(dir string) ([]*ScheduledJob, error) {
	items, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scheduled: readdir %s: %w", dir, err)
	}
	var jobs []*ScheduledJob
	for _, item := range items {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".yaml") {
			continue
		}
		job, err := LoadScheduledFile(filepath.Join(dir, item.Name()))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
