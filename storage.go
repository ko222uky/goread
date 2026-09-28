package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type storedData struct {
	FeedURLs []string          `json:"feed_urls"`
	Titles   map[string]string `json:"titles"`
	Read     map[string]bool   `json:"read"`
}

// localData stores data.json in the current working directory instead of
// the user config directory, so it can be committed alongside a project.
var localData bool

func dataPath() string {
	if localData {
		return "data.json"
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "goread", "data.json")
}

func loadData() (*storedData, error) {
	d := &storedData{
		Titles: make(map[string]string),
		Read:   make(map[string]bool),
	}
	b, err := os.ReadFile(dataPath())
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, nil
	}
	if err := json.Unmarshal(b, d); err != nil {
		return d, nil
	}
	if d.Titles == nil {
		d.Titles = make(map[string]string)
	}
	if d.Read == nil {
		d.Read = make(map[string]bool)
	}
	return d, nil
}

func saveData(d *storedData) error {
	path := dataPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
