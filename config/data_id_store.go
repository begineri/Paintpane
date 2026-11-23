//go:build msstore

package config

// DataID names this build's own data: the Fyne app ID, so the settings file
// %APPDATA%\fyne\<DataID>\preferences.json, and the folder under %LOCALAPPDATA% that holds the
// cache, the favourites and the log.
//
// The Microsoft Store (MSIX) build uses a name of its own, so it never reads or changes the
// settings, favourites or cache of a portable exe on the same PC (docs/store-feasibility.md §7).
// Inside the package Windows writes a folder that does not exist yet into the package's private
// AppData and deletes it when the app is uninstalled. The single-instance mutex and the local
// API port stay shared on purpose: only one of the two runs at a time.
const DataID = "PaintpaneStore"
