package assets

import (
	_ "embed"
	"os"
	"path/filepath"
)

// The embedded APKs come from the velo-platform release pinned in
// VELO_PLATFORM_VERSION; build/ci-build.sh and build/build.bat download them.

//go:embed velolauncher.apk
var embeddedVeloLauncher []byte

//go:embed serialbridge.apk
var embeddedSerialBridge []byte

//go:embed logo.png
var EmbeddedLogo []byte

// APKDirEnv names an environment variable that, when set, points FindAPK at a
// directory of APKs to install instead of the embedded ones (for testing
// local velo-platform builds). It is opt-in so a stray velolauncher.apk lying
// next to freewheel.exe, e.g. in Downloads, is never installed by accident.
const APKDirEnv = "FREEWHEEL_APK_DIR"

// FindAPK returns a path to the named APK and whether it is a local override.
// It uses $FREEWHEEL_APK_DIR/<name>.apk when that variable is set, otherwise
// it extracts the embedded APK to a temp file. It returns "" if the APK is
// unavailable, including when the override directory lacks it.
func FindAPK(name string) (path string, override bool) {
	if dir := os.Getenv(APKDirEnv); dir != "" {
		p := filepath.Join(dir, name+".apk")
		if _, err := os.Stat(p); err != nil {
			return "", true
		}
		return p, true
	}

	var data []byte
	switch name {
	case "velolauncher":
		data = embeddedVeloLauncher
	case "serialbridge":
		data = embeddedSerialBridge
	}
	if len(data) > 0 {
		tmp := filepath.Join(os.TempDir(), name+".apk")
		if err := os.WriteFile(tmp, data, 0644); err == nil {
			return tmp, false
		}
	}

	return "", false
}
