package device

import (
	"fmt"
	"strings"

	"github.com/ril3y/bowflex-tool/internal/adb"
)

// PreflightResult is the outcome of PreflightCheck.
type PreflightResult int

const (
	// PreflightBlocked: incompatible device, or it couldn't be checked.
	PreflightBlocked PreflightResult = iota
	// PreflightPassed: compatible, safe to jailbreak.
	PreflightPassed
	// PreflightNeedsOverride: compatible Nautilus hardware without JRNY,
	// e.g. a factory-reset bike that never ran the Nautilus setup. The user
	// must confirm, because Restore Stock can't reinstall JRNY afterwards.
	PreflightNeedsOverride
)

// PreflightCheck queries the device for version info, platform key, and compatibility.
func PreflightCheck(ip string, log Logger) PreflightResult {
	log.Step("[PRE-FLIGHT] Device compatibility check")

	connect := func() (*adb.Conn, error) {
		return adb.Connect(ip, 5555, log.Warn)
	}

	// --- Device info (single batched shell command to avoid ADB staleness) ---
	conn, err := connect()
	if err != nil {
		log.Error(fmt.Sprintf("  Connection failed: %v", err))
		return PreflightBlocked
	}
	propsRaw, _ := conn.Shell(
		"getprop ro.product.model; echo '|||'; " +
			"getprop ro.product.board; echo '|||'; " +
			"getprop ro.build.version.release; echo '|||'; " +
			"getprop ro.build.version.sdk; echo '|||'; " +
			"getprop ro.build.version.security_patch; echo '|||'; " +
			"getprop ro.serialno; echo '|||'; " +
			"getprop ro.build.fingerprint; echo '|||'; " +
			"uname -r; echo '|||'; " +
			"getprop ro.hardware; echo '|||'; " +
			"getprop ro.board.platform")
	conn.Close()

	propFields := strings.Split(propsRaw, "|||")
	getProp := func(i int) string {
		if i < len(propFields) {
			return strings.TrimSpace(propFields[i])
		}
		return ""
	}
	model := getProp(0)
	board := getProp(1)
	androidVer := getProp(2)
	sdkVer := getProp(3)
	secPatch := getProp(4)
	serial := getProp(5)
	fingerprint := getProp(6)
	kernel := getProp(7)
	chipset := getProp(8)
	soc := getProp(9)

	// Normalize Android version from SDK level
	if sdkVer == "28" || sdkVer == "9" {
		if sdkVer == "28" {
			androidVer = "9"
		}
	} else if secPatch == "9" || secPatch == "28" {
		androidVer = "9"
	}
	if androidVer != "9" {
		var sdkInt int
		fmt.Sscanf(sdkVer, "%d", &sdkInt)
		if sdkInt == 28 {
			androidVer = "9"
		}
	}

	// Board fallback: use ro.hardware or ro.board.platform if board is empty
	if board == "" {
		if chipset != "" {
			board = chipset
		} else if soc != "" {
			board = soc
		}
	}

	log.Info("  Device Information:")
	log.Dim(fmt.Sprintf("    Model: %s  Board: %s", model, board))
	log.Dim(fmt.Sprintf("    Android: %s (SDK %s)  Patch: %s", androidVer, sdkVer, secPatch))
	log.Dim(fmt.Sprintf("    Serial: %s", serial))
	log.Dim(fmt.Sprintf("    Kernel: %s", kernel))
	log.Dim(fmt.Sprintf("    Build: %s", fingerprint))

	// --- Nautilus package versions (single batched command) ---
	log.Info("  Package Versions:")
	conn, err = connect()
	if err != nil {
		log.Error(fmt.Sprintf("  Connection failed: %v", err))
		return PreflightBlocked
	}

	nautilusPkgs := []struct{ pkg, label string }{
		{"com.nautilus.bowflex.usb", "JRNY"},
		{"com.nautilus.nautiluslauncher", "NautilusLauncher"},
		{"com.nautilus.g4assetmanager", "Asset Manager"},
		{"com.nautilus.nlssbcsystemsettings", "System Settings"},
		{"com.nautilus.nautilusfactoryreset", "Factory Reset"},
		{"com.nautilus.platform_hardwaretest", "Hardware Test"},
		{"com.nautilus.UtilityApp", "Utility App"},
		{"com.redbend.client", "Redbend OTA"},
	}

	var pkgCmds []string
	for _, p := range nautilusPkgs {
		pkgCmds = append(pkgCmds, fmt.Sprintf("dumpsys package %s 2>/dev/null | grep versionName | head -1", p.pkg))
	}
	pkgRaw, _ := conn.Shell(strings.Join(pkgCmds, "; echo '|||'; "))
	conn.Close()

	pkgResults := strings.Split(pkgRaw, "|||")
	jrnyVersion := ""
	launcherVersion := ""
	for i, p := range nautilusPkgs {
		ver := ""
		if i < len(pkgResults) {
			ver = strings.TrimSpace(pkgResults[i])
		}
		if idx := strings.Index(ver, "="); idx >= 0 {
			ver = ver[idx+1:]
		}
		if ver == "" {
			ver = "not installed"
		}
		log.Dim(fmt.Sprintf("    %s: %s", p.label, ver))
		if p.pkg == "com.nautilus.bowflex.usb" {
			jrnyVersion = ver
		}
		if p.pkg == "com.nautilus.nautiluslauncher" {
			launcherVersion = ver
		}
	}

	// --- Platform signing key ---
	log.Info("  Platform Key:")
	keyMatch := false
	log.Dim("    Platform key will be verified when SerialBridge installs")
	log.Dim("    (SerialBridge is platform-signed — install fails if key doesn't match)")

	// --- Compatibility verdict ---
	log.Info("")
	log.Info("  Compatibility Verdict:")

	blocked := false

	if androidVer != "9" {
		log.Error(fmt.Sprintf("    BLOCKED: Android %s — only Android 9 is supported", androidVer))
		blocked = true
	}

	boardLower := strings.ToLower(board)
	if strings.Contains(boardLower, "rk3") || strings.Contains(boardLower, "rockchip") || strings.Contains(boardLower, "nftm") {
		log.Success(fmt.Sprintf("    Board: %s", board))
	} else if board == "" {
		log.Warn("    Board not detected — could not determine hardware platform")
		log.Dim("    Proceeding anyway (model and packages will be checked)")
	} else {
		log.Error(fmt.Sprintf("    BLOCKED: Board '%s' — only Rockchip platforms are supported", board))
		blocked = true
	}

	// Without JRNY, only Nautilus firmware is evidence this is a bike.
	nautilusFirmware := strings.Contains(strings.ToLower(fingerprint), "nautilus") ||
		strings.HasPrefix(strings.ToUpper(model), "NFTM")
	missingJRNY := false
	if jrnyVersion == "not installed" {
		if nautilusFirmware {
			log.Warn("    JRNY not installed, but this is Nautilus firmware (factory-reset bike?)")
			missingJRNY = true
		} else {
			log.Error("    BLOCKED: JRNY not installed and not Nautilus firmware — is this a Bowflex bike?")
			blocked = true
		}
	} else if jrnyVersion != "" {
		knownGood := "2.25.1"
		cmp := compareVersions(jrnyVersion, knownGood)
		if cmp == 0 {
			log.Success(fmt.Sprintf("    JRNY %s — exact match with tested version", jrnyVersion))
		} else if cmp < 0 {
			log.Warn(fmt.Sprintf("    JRNY %s — older than tested version (%s)", jrnyVersion, knownGood))
			log.Dim("    Likely compatible but not tested. Proceed with caution.")
		} else {
			log.Warn(fmt.Sprintf("    JRNY %s — NEWER than tested version (%s)", jrnyVersion, knownGood))
			log.Warn("    Nautilus may have changed internals. Proceed with caution.")
		}
	}

	if launcherVersion == "not installed" {
		log.Dim("    NautilusLauncher not installed (not needed — SerialBridge handles serial)")
	}

	if keyMatch {
		log.Success("    Platform key verified — SerialBridge will run as system")
	} else {
		log.Warn("    Platform key could not be verified pre-install")
		log.Dim("    If SerialBridge install fails, the key doesn't match and jailbreak is not possible")
	}

	if blocked {
		log.Error("")
		log.Error("  PRE-FLIGHT FAILED — Jailbreak blocked. Device is not compatible.")
		return PreflightBlocked
	}

	if missingJRNY {
		log.Warn("")
		log.Warn("  PRE-FLIGHT NEEDS CONFIRMATION — hardware is compatible, but JRNY is not installed")
		log.Warn("  Restore Stock can't reinstall JRNY. Going back to stock would need a factory reset.")
		log.Info("")
		return PreflightNeedsOverride
	}

	log.Success("")
	log.Success("  PRE-FLIGHT PASSED — Device is compatible")
	log.Info("")
	return PreflightPassed
}

// compareVersions compares two dot-separated version strings.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		var ai, bi int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &ai)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
