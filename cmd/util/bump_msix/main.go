// Command bump_msix writes the app version into an MSIX manifest:
//
//	go run cmd/util/bump_msix/main.go v1.2.0 dist/msix/AppxManifest.xml
//
// sets the Identity Version to 1.2.0.0. The Store requires four parts and the last one to be 0.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// identityVersion matches the Version attribute of the Identity element only: the whitespace in
// front keeps it off MinVersion and MaxVersionTested.
var identityVersion = regexp.MustCompile(`(<Identity\b[^>]*\s)Version="[0-9.]+"`)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: bump_msix <version> <manifest>")
		os.Exit(2)
	}
	version, err := msixVersion(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := setVersion(os.Args[2], version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("MSIX version %s\n", version)
}

// msixVersion turns "v1.2.0" (version.txt) into "1.2.0.0".
func msixVersion(v string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("version %q is not major.minor.patch", v)
	}
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err != nil || n < 0 || n > 65535 {
			return "", fmt.Errorf("version %q: %q is not a number from 0 to 65535", v, p)
		}
	}
	return strings.Join(parts, ".") + ".0", nil
}

func setVersion(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !identityVersion.Match(data) {
		return fmt.Errorf("%s: no Identity Version attribute", path)
	}
	data = identityVersion.ReplaceAll(data, []byte(`${1}Version="`+version+`"`))
	return os.WriteFile(path, data, 0o644)
}
