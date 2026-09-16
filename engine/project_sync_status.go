package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Reports the helper's last completed check, never a claim about unsaved edits.
func projectSyncStatus(root, support string, now time.Time) syncView {
	var config struct {
		Roots []struct {
			ID, Path string
			Cloud    bool
		}
	}
	raw, err := os.ReadFile(filepath.Join(support, "remote.json"))
	if err != nil || json.Unmarshal(raw, &config) != nil {
		return syncView{}
	}
	canonical, _ := filepath.EvalSymlinks(root)
	for _, r := range config.Roots {
		candidate, _ := filepath.EvalSymlinks(r.Path)
		if canonical == "" || canonical != candidate {
			continue
		}
		view := syncView{Enabled: true, Mode: "project", Message: "JADE cloud sync is off for this folder. Local saves do not confirm delivery to your phone."}
		if !r.Cloud {
			return view
		}
		view.Message = "JADE cloud sync enabled · waiting for the Mac Connection helper."
		var state struct {
			CheckedAt float64
			Projects  map[string]string
		}
		raw, err = os.ReadFile(filepath.Join(support, "cloud-status.json"))
		if err != nil || json.Unmarshal(raw, &state) != nil || state.CheckedAt <= 0 {
			return view
		}
		checked := time.UnixMilli(int64(state.CheckedAt * 1000))
		view.LastSync = checked.Format(time.RFC3339)
		if now.Sub(checked) > time.Minute {
			view.Message = "JADE cloud sync has not checked recently. Open JaDE Mac Connection."
			return view
		}
		if status := state.Projects[r.ID]; status != "" {
			view.Message = "Last JADE cloud check: " + status
		}
		return view
	}
	return syncView{}
}
