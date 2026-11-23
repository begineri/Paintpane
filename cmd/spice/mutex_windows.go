//go:build windows
// +build windows

package main

import (
	"errors"
	"syscall"

	"github.com/dixieflatline76/Spice/v2/util/log"

	"github.com/dixieflatline76/Spice/v2/config"
	"golang.org/x/sys/windows"
)

var (
	mutex windows.Handle
)

// acquireLock tries to acquire a single-instance lock (mutex on Windows).
func acquireLock() (bool, error) {
	namePtr, err := syscall.UTF16PtrFromString(config.AppName + "_SingleInstanceMutex")
	if err != nil {
		return false, err
	}

	mutex, err = windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		// The handle of a mutex that already exists keeps it alive: close it, so the mutex goes
		// away with the instance that made it (a start that waits for it retries).
		if mutex != 0 {
			_ = windows.CloseHandle(mutex)
			mutex = 0
		}
		// Check if the error is because the mutex already exists.
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return false, nil // Another instance is running
		}
		return false, errors.New("another instance is already running")
	}

	return true, nil
}

// instanceRunning reports whether another instance holds the single-instance mutex, without
// taking it.
func instanceRunning() bool {
	namePtr, err := syscall.UTF16PtrFromString(config.AppName + "_SingleInstanceMutex")
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, namePtr)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

// releaseLock releases the single-instance lock handle.
func releaseLock() {
	if mutex != 0 { // Important check
		// Remove this call - ReleaseMutex is not needed when only using CreateMutex for existence check
		/*
		   err := windows.ReleaseMutex(mutex)
		   if err != nil {
		       log.Printf("Failed to release mutex %v", err)
		   }
		*/
		err := windows.CloseHandle(mutex) // This is the important cleanup call
		if err != nil {
			log.Printf("Failed to close mutex handle: %v", err)
		}
		mutex = 0 // Optional: Clear the handle variable after closing
	}
}
