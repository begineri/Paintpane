package wallpaper

import "sort"

// DebugImage is the part of an image that is useful when checking the app from outside.
type DebugImage struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// DebugMonitor is one display and what it shows.
type DebugMonitor struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Paused      bool       `json:"paused"`
	Initialized bool       `json:"initialized"`
	Image       DebugImage `json:"image"`
	// LastSetError is why the last attempt to set the desktop wallpaper failed; empty when it worked.
	LastSetError string `json:"lastSetError,omitempty"`
}

// DebugState is a read-only snapshot of the wallpaper plugin for dev tooling.
type DebugState struct {
	DisplayMode string `json:"displayMode"`
	ShowNSFW    bool   `json:"showNSFW"`
	// ActiveQueries counts active queries (collections, searches) per provider ID.
	ActiveQueries map[string]int `json:"activeQueries"`
	Monitors      []DebugMonitor `json:"monitors"`
}

// DebugState returns a snapshot of the plugin's state. It only reads.
func (wp *Plugin) DebugState() DebugState {
	st := DebugState{ActiveQueries: map[string]int{}}
	if wp.cfg != nil {
		st.DisplayMode = wp.cfg.GetArtworkDisplayMode().String()
		st.ShowNSFW = wp.cfg.GetShowNudes()
		for _, q := range wp.cfg.GetQueries() {
			if q.Active {
				st.ActiveQueries[q.Provider]++
			}
		}
	}

	wp.monMu.RLock()
	for id, mc := range wp.Monitors {
		mc.mu.RLock()
		img := mc.State.CurrentImage
		st.Monitors = append(st.Monitors, DebugMonitor{
			ID:          id,
			Name:        mc.Monitor.Name,
			Paused:      mc.State.Paused,
			Initialized: mc.State.CurrentID != "" || img.ID != "",
			Image: DebugImage{
				ID:          img.ID,
				Provider:    img.Provider,
				Title:       img.Title,
				Artist:      img.Artist,
				Attribution: img.Attribution,
				FilePath:    img.FilePath,
				Width:       img.Width,
				Height:      img.Height,
			},
			LastSetError: mc.State.LastSetError,
		})
		mc.mu.RUnlock()
	}
	wp.monMu.RUnlock()

	sort.Slice(st.Monitors, func(i, j int) bool { return st.Monitors[i].ID < st.Monitors[j].ID })
	return st
}
