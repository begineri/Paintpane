//go:build !msstore

package config

// DataID names this build's own data: the Fyne app ID, so the settings file
// %APPDATA%\fyne\<DataID>\preferences.json, and the folder under %LOCALAPPDATA% that holds the
// cache, the favourites and the log. The portable exe uses the app name.
const DataID = AppName
