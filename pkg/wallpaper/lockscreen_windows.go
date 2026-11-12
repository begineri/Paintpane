//go:build windows
// +build windows

package wallpaper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	lockScreenImageEnv = "SPICE_LOCKSCREEN_IMAGE"
	lockScreenTimeout  = 60 * time.Second

	// The image path arrives through the environment, so it is never parsed as script.
	setLockScreenScript = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime]
$null = [Windows.System.UserProfile.LockScreen, Windows.System.UserProfile, ContentType = WindowsRuntime]
$methods = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 }
$asOperation = $methods | Where-Object { $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' } | Select-Object -First 1
$asAction = $methods | Where-Object { $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncAction' } | Select-Object -First 1
$fileOperation = [Windows.Storage.StorageFile]::GetFileFromPathAsync($env:SPICE_LOCKSCREEN_IMAGE)
$fileTask = $asOperation.MakeGenericMethod([Windows.Storage.StorageFile]).Invoke($null, @($fileOperation))
$null = $fileTask.Wait(-1)
$setTask = $asAction.Invoke($null, @([Windows.System.UserProfile.LockScreen]::SetImageFileAsync($fileTask.Result)))
$null = $setTask.Wait(-1)
`
)

// SetLockScreen sets the Windows lock screen image. It uses a hidden PowerShell process
// so that a WinRT failure cannot take down the application.
func (w *windowsOS) SetLockScreen(imagePath string) error {
	absPath, err := filepath.Abs(resolveMSIXPicturePath(imagePath))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), lockScreenTimeout)
	defer cancel()

	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	// #nosec G204 -- fixed executable and script; the image path is passed through the environment
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", setLockScreenScript)
	cmd.Env = append(os.Environ(), lockScreenImageEnv+"="+absPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("powershell failed for %s: %w: %s", absPath, err, strings.TrimSpace(string(output)))
	}
	return nil
}
