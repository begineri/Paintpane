# --- Extract version needed for EULA hash ---
VERSION := $(shell sh -c "cat version.txt" 2> /dev/null || cmd /c "type version.txt")

# --- Build flags ---
LDFLAGS_SECRETS := $(shell go run cmd/util/load_secrets/main.go)
LDFLAGS_COMMON := -X main.version=$(VERSION) $(LDFLAGS_SECRETS)
# Release builds drop the symbol table and DWARF debug info (-s -w): about 23 MB smaller. Panics
# still print function names and line numbers. Development builds keep them.
LDFLAGS_RELEASE := -s -w $(LDFLAGS_COMMON)
BUILD_NUMBER ?= 1
WIN_SDK_VERSION := $(shell pwsh -NoProfile -Command "try { (Get-ChildItem 'C:/Program Files (x86)/Windows Kits/10/bin/10.*' -Directory | Sort-Object Name -Descending | Select-Object -First 1).Name } catch { '10.0.28000.0' }" 2>/dev/null || echo 10.0.28000.0)
MAKEAPPX := "C:\Program Files (x86)\Windows Kits\10\bin\$(WIN_SDK_VERSION)\x64\makeappx.exe"
SIGNTOOL := "C:\Program Files (x86)\Windows Kits\10\bin\$(WIN_SDK_VERSION)\x64\signtool.exe"
PFX_PATH ?= 
PFX_PASSWORD ?= SpicePassword123

# Prevent MSYS2/Git-for-Windows bash from converting /d to D:/ etc.
export MSYS_NO_PATHCONV := 1

# --- Code Generation ---
gen-i18n:
	go run cmd/util/gen_i18n/main.go

check-i18n:
	go run cmd/util/gen_i18n/main.go --check

generate: gen-i18n
	go generate ./...
	go run cmd/gallerygen/main.go

# --- Build targets ---
build-win-amd64:
	set GOOS=windows&& set GOARCH=amd64&& go build -tags release -o bin/paintpane.exe -ldflags "-H=windowsgui $(LDFLAGS_RELEASE)" ./cmd/spice

build-win-console-amd64:
	set GOOS=windows&& set GOARCH=amd64&& go build -tags release -o bin/paintpane-console.exe -ldflags "$(LDFLAGS_RELEASE)" ./cmd/spice

build-win-arm64:
	set GOOS=windows&& set GOARCH=arm64&& go build -tags release -o bin/paintpane-arm64.exe -ldflags "-H=windowsgui $(LDFLAGS_RELEASE)" ./cmd/spice

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -tags release -o bin/paintpane-amd64 -ldflags "$(LDFLAGS_RELEASE)" ./cmd/spice

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -tags release -o bin/paintpane-arm64 -ldflags "$(LDFLAGS_RELEASE)" ./cmd/spice

build-darwin-amd64:
	@echo "Building Go executable for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 go build -tags release -o bin/paintpane-darwin-amd64 -ldflags "$(LDFLAGS_RELEASE)" ./cmd/spice

	@echo "Cleaning previous Paintpane.app bundle..."
	rm -rf Paintpane.app
	@echo "Packaging Paintpane.app..."
	fyne package --os darwin --executable ./bin/paintpane-darwin-amd64 --icon asset/icons/tray.png --name Paintpane --app-id io.github.begineri.paintpane

	@echo "Modifying Info.plist to set LSUIElement=true..."
	plutil -insert LSUIElement -bool true Paintpane.app/Contents/Info.plist

	@echo "Moving final Paintpane.app to ./bin/..."
	rm -rf ./bin/Paintpane.app && mv Paintpane.app ./bin/

build-darwin-arm64:
	@echo "Building Go executable for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 go build -tags release -o bin/paintpane-darwin-arm64 -ldflags "$(LDFLAGS_RELEASE)" ./cmd/spice

	@echo "Cleaning previous Paintpane.app bundle..."
	rm -rf Paintpane.app
	@echo "Packaging Paintpane.app..."
	fyne package --os darwin --executable ./bin/paintpane-darwin-arm64 --icon asset/icons/tray.png --name Paintpane --app-id io.github.begineri.paintpane

	@echo "Modifying Info.plist to set LSUIElement=true..."
	plutil -insert LSUIElement -bool true Paintpane.app/Contents/Info.plist

ifneq ($(SIGNING_IDENTITY),)
	@echo "Signing image processor binary..."
	codesign --force --options=runtime --sign "$(SIGNING_IDENTITY)" --timestamp bin/paintpane-darwin-arm64
	@echo "Signing the application bundle..."
	codesign --force --options=runtime --sign "$(SIGNING_IDENTITY)" --timestamp Paintpane.app
	@echo "Verifying application bundle signature..."
	codesign -dv --verbose=4 Paintpane.app
else
	@echo "No SIGNING_IDENTITY set, skipping code signing."
endif

	@echo "Moving final Paintpane.app to ./bin/..."
	rm -rf ./bin/Paintpane.app && mv Paintpane.app ./bin/

# --- Development build targets ---
build-win-amd64-dev:
	set GOOS=windows&& set GOARCH=amd64&& go build -o bin/paintpane-dev.exe -ldflags "-H=windowsgui $(LDFLAGS_COMMON)" ./cmd/spice

build-win-console-amd64-dev:
	set GOOS=windows&& set GOARCH=amd64&& go build -o bin/paintpane-console-dev.exe -ldflags "$(LDFLAGS_COMMON)" ./cmd/spice

build-linux-amd64-dev:
	GOOS=linux GOARCH=amd64 go build -o bin/paintpane-amd64 -ldflags "$(LDFLAGS_COMMON)" ./cmd/spice

build-linux-arm64-dev:
	GOOS=linux GOARCH=arm64 go build -o bin/paintpane-arm64 -ldflags "$(LDFLAGS_COMMON)" ./cmd/spice

build-darwin-amd64-dev:
	@echo "Building Go executable for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 go build -o bin/paintpane-darwin-amd64 -ldflags "$(LDFLAGS_COMMON)" ./cmd/spice

	@echo "Packaging Paintpane.app..."
	fyne package --os darwin --executable ./bin/paintpane-darwin-amd64 --icon asset/icons/tray.png --name Paintpane --app-id io.github.begineri.paintpane

	@echo "Modifying Info.plist to set LSUIElement=true..."
	plutil -insert LSUIElement -bool true Paintpane.app/Contents/Info.plist

	@echo "Moving final Paintpane.app to ./bin/..."
	rm -rf ./bin/Paintpane.app && mv Paintpane.app ./bin/

build-darwin-arm64-dev:
	@echo "Building Go executable for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 go build -o bin/paintpane-darwin-arm64 -ldflags "$(LDFLAGS_COMMON)" ./cmd/spice

	@echo "Packaging Paintpane.app..."
	fyne package --os darwin --executable ./bin/paintpane-darwin-arm64 --icon asset/icons/tray.png --name Paintpane --app-id io.github.begineri.paintpane

	@echo "Modifying Info.plist to set LSUIElement=true..."
	plutil -insert LSUIElement -bool true Paintpane.app/Contents/Info.plist

	@echo "Moving final Paintpane.app to ./bin/..."
	rm -rf ./bin/Paintpane.app && mv Paintpane.app ./bin/

# --- Other targets ---
lint:
	gofmt -w .
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --timeout=10m ./...

security:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --timeout=10m --enable gosec --tests=false ./...

test:
	go test ./...

# Render every preferences page (zh/en/de, light/dark) to PNG without opening a window.
# SHOTS sets the output folder. The wallpaper plugin writes its cache under a throwaway LOCALAPPDATA.
SHOTS ?= screenshots
screenshots:
	mkdir -p "$(SHOTS)"
	LOCALAPPDATA="$$(mktemp -d)" SCREENSHOT_DIR="$(abspath $(SHOTS))" go test -tags screenshots ./ui/ -run TestPreferencesScreenshots -count=1

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

update-patch-deps:
	go get -u=patch ./... 
	go mod tidy

update-minor-deps:
	go get -u ./...
	go mod tidy

list-updates:
	@echo "Checking for all available updates (including major)..."
	go list -m -u all
	@echo "Review the list above. Update major versions manually using 'go get module/vX@latest'."

# --- Main build targets ---
win-amd64: generate update-patch-deps check-i18n lint test build-win-amd64 build-win-console-amd64

win-amd64-dev: generate update-patch-deps lint test build-win-amd64-dev build-win-console-amd64-dev

linux-amd64: generate update-patch-deps check-i18n lint test build-linux-amd64

linux-amd64-dev: generate update-patch-deps lint test build-linux-amd64-dev

darwin-amd64: generate update-patch-deps check-i18n lint test build-darwin-amd64

darwin-amd64-dev: generate update-patch-deps lint test build-darwin-amd64-dev

darwin-arm64: generate update-patch-deps check-i18n lint test build-darwin-arm64

darwin-arm64-dev: generate update-patch-deps lint test build-darwin-arm64-dev

# --- Clean target (cross-platform) ---
clean:
ifeq ($(OS),Windows_NT)
	if exist bin rmdir /s /q bin
	if exist coverage* del /q coverage*
	if exist *.out del /q *.out
	if exist coverage.html del /q coverage.html
	-del /s /q asset\galleries\*.html >nul 2>&1
	-del /s /q asset\galleries\*.json >nul 2>&1
	if not exist asset\galleries\dummy mkdir asset\galleries\dummy
	type nul > asset\galleries\dummy\keep.html
else
	$(RM) -r bin coverage* *.out coverage.html asset/galleries/*/*.html asset/galleries/*/*.json asset/galleries/*.json
	mkdir -p asset/galleries/dummy
	touch asset/galleries/dummy/keep.html
endif
	go clean

# Combined coverage target
coverage:
	go test -v -coverprofile=coverage.out ./...
	@echo "Filtering coverage report..."
	@grep -v "cmd/util" coverage.out > coverage.filtered.out
	@move /Y coverage.filtered.out coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out > docs/coverage_report.md
	@echo "Coverage report generated: coverage.html & docs/coverage_report.md"
	@echo "Summary:"
	@go tool cover -func=coverage.out

# Legacy aliases
test-coverage: coverage
coverage-report: coverage

# Define the command based on OS
# Default for Linux/macOS
CREATE_DIR_CMD := mkdir -p bin/util

# Override for Windows
ifeq ($(OS),Windows_NT)
  # Use cmd /c for native Windows commands. Use backslashes inside the cmd string for safety.
  CREATE_DIR_CMD := cmd /c "(if not exist bin mkdir bin) && (if not exist bin\util mkdir bin\util)"
endif

# Build rule for the version_bump utility
bin/util/version_bump: ./cmd/util/version_bump.go
	@echo "Building version_bump utility..."
	$(CREATE_DIR_CMD)
	go build -o $@ $<

build-version-bump: bin/util/version_bump
	@echo "version_bump utility is up-to-date."

# Bump rules, now depending on build-version-bump
bump-patch: build-version-bump
	@echo "Bumping patch version..."
	./bin/util/version_bump patch

bump-minor: build-version-bump
	@echo "Bumping minor version..."
	./bin/util/version_bump minor

bump-major: build-version-bump
	@echo "Bumping major version..."
	./bin/util/version_bump major

.PHONY: build-win-amd64 build-win-console-amd64 build-win-arm64 build-linux-amd64 build-darwin-amd64 build-darwin-arm64 build-win-amd64-dev build-win-console-amd64-dev build-linux-amd64-dev gen-i18n check-i18n lint test screenshots update-patch-deps update-minor-deps list-updates win-amd64 win-amd64-dev linux-amd64 linux-amd64-dev darwin-amd64 darwin-amd64-dev darwin-arm64 darwin-arm64-dev clean build-version-bump bump-patch bump-minor bump-major coverage
