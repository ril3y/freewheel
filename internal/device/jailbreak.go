package device

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ril3y/bowflex-tool/internal/adb"
)

// FindAPKFunc locates an APK by name and reports whether it is a local
// override rather than the embedded copy. Set by the caller to avoid a
// circular dependency with the assets package.
var FindAPKFunc func(name string) (path string, override bool)

// RunJailbreak executes the full jailbreak sequence on the device.
func RunJailbreak(ip string, log Logger) {
	totalSteps := 9

	step := func(n int, name string) {
		log.Step(fmt.Sprintf("[STEP %d/%d] %s", n, totalSteps, name))
	}

	connect := func() (*adb.Conn, error) {
		return adb.Connect(ip, 5555, log.Warn)
	}

	// installAPK pushes and installs an APK by name. Returns true on success.
	// It updates in place ("pm install -r") and only uninstalls the old
	// version when Android refuses the update (different signing key, e.g.
	// the v1.4.0 debug-signed launcher, or a newer version already
	// installed), so a dropped connection never leaves the app missing.
	installAPK := func(name, pkg string) bool {
		apkPath, override := FindAPKFunc(name)
		if apkPath == "" {
			log.Error(fmt.Sprintf("  %s.apk not found!", name))
			return false
		}
		if override {
			log.Warn(fmt.Sprintf("  Using LOCAL OVERRIDE (FREEWHEEL_APK_DIR): %s", apkPath))
		} else {
			log.Dim(fmt.Sprintf("  Using: %s", apkPath))
		}

		// Push APK
		conn, err := connect()
		if err != nil {
			log.Error(fmt.Sprintf("  Connection failed: %v", err))
			return false
		}
		tmpPath := fmt.Sprintf("/data/local/tmp/_%s.apk", name)
		err = conn.Push(apkPath, tmpPath, 0644)
		conn.Close()
		if err != nil {
			log.Error(fmt.Sprintf("  Push failed: %v", err))
			return false
		}

		install := func() (string, bool) {
			conn, err := connect()
			if err != nil {
				log.Error(fmt.Sprintf("  Connection failed: %v", err))
				return "", false
			}
			defer conn.Close()
			conn.Shell(fmt.Sprintf("am force-stop %s 2>/dev/null", pkg))
			out, err := conn.Shell(fmt.Sprintf("pm install -r %s 2>&1", tmpPath))
			if err != nil && !strings.Contains(out, "Success") {
				out = strings.TrimSpace(out + fmt.Sprintf("\n(%v)", err))
			}
			return out, true
		}

		out, ok := install()
		if !ok {
			return false
		}
		if !strings.Contains(out, "Success") && installNeedsUninstall(out) {
			log.Warn(fmt.Sprintf("  Can't update %s in place (%s) — uninstalling the old version first", pkg, installFailureCode(out)))
			conn, err := connect()
			if err != nil {
				log.Error(fmt.Sprintf("  Connection failed: %v", err))
				return false
			}
			conn.Shell(fmt.Sprintf("pm uninstall %s 2>/dev/null", pkg))
			conn.Close()
			if out, ok = install(); !ok {
				return false
			}
		}
		outTrimmed := strings.TrimSpace(out)
		if !strings.Contains(out, "Success") {
			if outTrimmed == "" {
				outTrimmed = "(no output — install may have timed out)"
			}
			log.Error(fmt.Sprintf("  Install failed: %s", outTrimmed))
			return false
		}

		// Cleanup temp file
		conn, err = connect()
		if err == nil {
			conn.Shell(fmt.Sprintf("rm %s", tmpPath))
			conn.Close()
		}
		return true
	}

	// --- Step 1: Connect ---
	step(1, "Connecting to bike")
	log.Dim(fmt.Sprintf("  Target: %s:5555", ip))

	conn, err := connect()
	if err != nil {
		log.Error(fmt.Sprintf("  Connection failed: %v", err))
		log.Warn("  Make sure: 1) Bike is on same WiFi  2) Utility App was opened (9-tap)")
		return
	}
	log.Success("  Connected!")

	// --- Step 2: Verify ---
	step(2, "Verifying ADB shell")
	out, err := conn.Shell("id")
	if err != nil {
		log.Error(fmt.Sprintf("  Shell failed: %v", err))
		conn.Close()
		return
	}
	idStr := strings.TrimSpace(out)
	if idx := strings.Index(idStr, " groups="); idx > 0 {
		idStr = idStr[:idx]
	}
	log.Success(fmt.Sprintf("  %s", idStr))
	conn.Close()

	// Check if already jailbroken — if so, this is an update, not a fresh
	// jailbreak. If this can't be checked, stop before changing anything:
	// guessing "fresh" would auto-restore a working jailbroken bike on failure.
	conn, err = connect()
	if err != nil {
		log.Error(fmt.Sprintf("  Could not check whether the bike is already jailbroken: %v", err))
		log.Warn("  Nothing was changed. Run Jailbreak again.")
		return
	}
	pkgCheck, err := conn.Shell("pm list packages 2>/dev/null | grep -E 'freewheel.launcher|freewheel.bridge'")
	conn.Close()
	if err != nil {
		log.Error(fmt.Sprintf("  Could not check whether the bike is already jailbroken: %v", err))
		log.Warn("  Nothing was changed. Run Jailbreak again.")
		return
	}
	isUpdate := strings.Contains(pkgCheck, "freewheel.launcher")
	if isUpdate {
		log.Info("  Already jailbroken — running as UPDATE (will reinstall APKs)")
	}

	// If anything fails on a fresh jailbreak, auto-restore to stock.
	// On an update, don't auto-restore (the device is already in a working state).
	failed := false
	defer func() {
		if failed && !isUpdate {
			log.Step("")
			log.Error("=== JAILBREAK FAILED — RESTORING STOCK ===")
			log.Warn("  A step failed. Automatically restoring bike to stock state...")
			log.Info("")
			RunRestore(ip, log)
		} else if failed && isUpdate {
			log.Step("")
			log.Warn("=== UPDATE HAD ERRORS ===")
			log.Warn("  Some steps failed on an already-jailbroken bike. Check the log above for details.")
			reportUpdateFailureState(connect, log)
		}
	}()

	fail := func(msg string) {
		log.Error(msg)
		failed = true
	}

	// skipped lists non-critical steps that couldn't run, for the summary.
	var skipped []string

	// --- Step 3: Disable JRNY + AppMonitor ---
	step(3, "Disabling JRNY and AppMonitor")
	conn, err = connect()
	if err != nil {
		fail(fmt.Sprintf("  Connection failed: %v", err))
		return
	}
	conn.Shell("am stop-service com.nautilus.nautiluslauncher/.thirdparty.appmonitor.AppMonitorService 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.nautilus.bowflex.usb 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.nautilus.g4assetmanager 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.nautilus.nlssbcsystemsettings 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.redbend.vdmc 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.redbend.client 2>/dev/null")
	conn.Shell("pm disable-user --user 0 com.redbend.dualpart.service.app 2>/dev/null")
	conn.Shell("settings put global software_update 0 2>/dev/null")
	conn.Shell("settings put global auto_update 0 2>/dev/null")
	conn.Close()
	log.Success("  JRNY, AppMonitor, OTA all disabled")

	// --- Step 4: Check current state ---
	step(4, "Checking current state")
	conn, err = connect()
	if err != nil {
		fail(fmt.Sprintf("  Connection failed: %v", err))
		return
	}
	out, _ = conn.Shell("pm list packages 2>/dev/null | grep -iE 'freewheel.launcher|freewheel.bridge'")
	conn.Close()

	if strings.Contains(out, "freewheel.launcher") {
		log.Dim("  VeloLauncher present -- will reinstall")
	}
	if strings.Contains(out, "freewheel.bridge") {
		log.Dim("  SerialBridge present -- will reinstall")
	}

	// --- Step 5: Install SerialBridge (critical) ---
	step(5, "Installing SerialBridge (takes over serial port)")
	if !installAPK("serialbridge", "io.freewheel.bridge") {
		fail("  SerialBridge install failed — cannot proceed without serial port control")
		return
	}
	log.Success("  SerialBridge installed (io.freewheel.bridge)")
	// Stock apps must be stopped BEFORE the bridge starts: if two processes
	// read /dev/ttyS4 at once, each gets partial frames and sensors look dead.
	conn, err = connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not reconnect to disable NautilusLauncher: %v", err))
	} else {
		conn.Shell("am force-stop com.nautilus.nautiluslauncher 2>/dev/null")
		conn.Shell("pm disable-user --user 0 com.nautilus.nautiluslauncher 2>/dev/null")
		conn.Close()
		log.Dim("  Checking stock serial apps are stopped...")
		stopStockSerialApps(connect, log)
	}
	conn, err = connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not reconnect to start SerialBridge: %v", err))
		log.Warn("  VeloLauncher starts it on next boot; reboot the bike after jailbreak if sensors stay at zero.")
		skipped = append(skipped, "start SerialBridge")
	} else {
		conn.Shell("am start-foreground-service -n io.freewheel.bridge/.BridgeService 2>/dev/null")
		conn.Close()
		log.Success("  NautilusLauncher disabled, SerialBridge started")
		// Re-check after the bridge is up, in case a stock app was respawned.
		time.Sleep(2 * time.Second)
		log.Dim("  Verifying stock serial apps stayed stopped...")
		stopStockSerialApps(connect, log)
	}

	// --- Step 6: Install VeloLauncher (critical) ---
	step(6, "Installing VeloLauncher")
	if !installAPK("velolauncher", "io.freewheel.launcher") {
		fail("  VeloLauncher install failed — cannot proceed without a launcher")
		return
	}
	log.Success("  VeloLauncher installed")
	conn, err = connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not reconnect to set home screen and permissions: %v", err))
		log.Warn("  Run Jailbreak again (it will update in place) to finish this step.")
		skipped = append(skipped, "set home screen and permissions")
	} else {
		conn.Shell("cmd package set-home-activity io.freewheel.launcher/.MainActivity")
		conn.Shell("pm disable-user --user 0 com.android.launcher3 2>/dev/null")
		conn.Shell("pm disable-user --user 0 com.android.quickstep 2>/dev/null")
		conn.Shell("appops set io.freewheel.launcher SYSTEM_ALERT_WINDOW allow")
		// Belt and braces: SYSTEM_ALERT_WINDOW is an appop on Android 9, so
		// pm grant usually refuses it. Harmless if it does.
		conn.Shell("pm grant io.freewheel.launcher android.permission.SYSTEM_ALERT_WINDOW 2>/dev/null")
		conn.Shell("pm grant io.freewheel.launcher android.permission.WRITE_SETTINGS 2>/dev/null")
		opOut, _ := conn.Shell("appops get io.freewheel.launcher SYSTEM_ALERT_WINDOW 2>/dev/null")
		conn.Close()
		log.Success("  Set as default home, other launchers disabled")
		switch {
		case overlayAllowed(opOut):
			log.Dim("  Overlay + settings permissions granted")
		case strings.TrimSpace(opOut) == "":
			log.Warn("  Could not verify overlay permission (floating home button / ride stats).")
			log.Warn("  If the overlay is missing, run: adb shell appops set io.freewheel.launcher SYSTEM_ALERT_WINDOW allow")
		default:
			log.Warn(fmt.Sprintf("  Overlay permission not granted (%s)", strings.TrimSpace(opOut)))
			log.Warn("  Fix manually: adb shell appops set io.freewheel.launcher SYSTEM_ALERT_WINDOW allow")
		}
	}

	// --- Step 7: Apply system settings ---
	step(7, "Applying system settings")
	conn, err = connect()
	if err == nil {
		// Disable Chrome only (saves RAM, safe to disable)
		// NOTE: Do NOT disable GMS or GSF — they're required for WiFi HAL
		// and Bluetooth stability on Android 9. Disabling them causes WiFi
		// to fail with "Operation not permitted" and BT to crash-loop.
		conn.Shell("pm disable-user --user 0 com.android.chrome 2>/dev/null")
		log.Dim("  Disabled: Chrome")
		conn.Shell("pm enable com.google.android.gms 2>/dev/null")
		conn.Shell("pm enable com.google.android.gsf 2>/dev/null")
		conn.Shell("pm enable com.android.vending 2>/dev/null")
		conn.Shell("pm enable com.google.android.webview 2>/dev/null")
		log.Success("  Google services, Play Store, WebView enabled")

		// Enable navbar, disable kiosk mode
		conn.Shell("settings put secure navigationbar_switch 1")
		conn.Shell("settings put global force_show_navbar 1")
		conn.Shell("settings put secure statusbar_switch 0")
		conn.Shell("settings put secure notification_switch 0")
		conn.Shell("settings put secure ntls_launcher_preference 0")
		conn.Shell("settings put global stay_on_while_plugged_in 3")
		log.Dim("  Navbar enabled, screen stays on while plugged in")
		conn.Close()
	} else {
		log.Warn(fmt.Sprintf("  Could not reconnect to apply system settings: %v", err))
		log.Warn("  Navbar and screen-on settings were not applied. Run Jailbreak again (it updates in place) to finish this step.")
		skipped = append(skipped, "apply system settings")
	}

	// --- Step 8: ADB persistence ---
	step(8, "Keeping ADB over WiFi enabled")
	// persist props survive reboots
	conn, err = connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not reconnect to set ADB persistence: %v", err))
		skipped = append(skipped, "keep ADB over WiFi enabled")
	} else {
		conn.Shell("setprop persist.sys.usb.config adb")
		conn.Shell("setprop persist.adb.tcp.port 5555")
		conn.Shell("settings put global adb_enabled 1")
		conn.Close()
		log.Dim("  ADB persistence set (persist.sys.usb.config, persist.adb.tcp.port)")
	}

	// --- Step 9: Final setup + sensor check ---
	step(9, "Final setup and sensor check")

	conn, err = connect()
	if err == nil {
		conn.Shell("input keyevent KEYCODE_HOME")
		conn.Close()
		log.Success("  Home screen active")
	} else {
		log.Warn(fmt.Sprintf("  Could not reconnect to show the home screen: %v", err))
	}

	checkBridgeHealth(connect, log)

	// Final summary
	log.Step("")
	if len(skipped) > 0 {
		log.Warn("=== JAILBREAK COMPLETE, WITH SKIPPED STEPS ===")
		log.Warn(fmt.Sprintf("  Lost the connection during: %s", strings.Join(skipped, ", ")))
		log.Warn("  Run Jailbreak again (it updates in place) to finish them. Intended result:")
	} else {
		log.Success("=== JAILBREAK COMPLETE ===")
	}
	log.Info("")
	log.Dim("  SerialBridge owns the bike serial port, serves TCP:9999 (NautilusLauncher disabled)")
	log.Dim("  VeloLauncher set as home screen (free, no subscription)")
	log.Dim("  JRNY, AppMonitor, OTA all disabled (persists across reboots)")
	log.Dim("  Chrome disabled, Google services kept enabled (required for WiFi/BT)")
	log.Dim("  ADB on port 5555, navbar enabled, kiosk mode off")
	log.Dim("  Screen stays on while plugged in")
	log.Info("")
	log.Info("VeloLauncher BootReceiver restarts itself + SerialBridge on reboot.")
	log.Info("All package disabling persists natively across Android reboots.")
}

// stockSerialPkgs are the stock apps that read the bike's serial port. If one
// is still running alongside SerialBridge, both get partial frames.
var stockSerialPkgs = []string{"com.nautilus.nautiluslauncher", "com.nautilus.bowflex.usb"}

// stopStockSerialApps verifies the stock serial apps are disabled and not
// running, stopping any that are. Only logs; never fails the jailbreak.
func stopStockSerialApps(connect func() (*adb.Conn, error), log Logger) {
	conn, err := connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not check stock apps: %v", err))
		return
	}
	defer conn.Close()

	disabledOut, _ := conn.Shell("pm list packages -d 2>/dev/null")
	disabled := map[string]bool{}
	for _, line := range strings.Split(disabledOut, "\n") {
		disabled[strings.TrimPrefix(strings.TrimSpace(line), "package:")] = true
	}
	pidof := func(pkg string) string {
		out, _ := conn.Shell(fmt.Sprintf("pidof %s 2>/dev/null", pkg))
		return strings.TrimSpace(out)
	}

	listed := strings.TrimSpace(disabledOut) != ""
	if !listed {
		log.Dim("  Could not list disabled packages; re-applying disable")
	}

	for _, pkg := range stockSerialPkgs {
		if !disabled[pkg] {
			if listed {
				log.Warn(fmt.Sprintf("  %s was not disabled — disabling it", pkg))
			}
			conn.Shell(fmt.Sprintf("pm disable-user --user 0 %s 2>/dev/null", pkg))
		}
		pid := pidof(pkg)
		if pid == "" {
			log.Dim(fmt.Sprintf("  %s: disabled, not running", pkg))
			continue
		}
		log.Warn(fmt.Sprintf("  %s still running (pid %s) — stopping it", pkg, pid))
		conn.Shell(fmt.Sprintf("am force-stop %s 2>/dev/null", pkg))
		conn.Shell(fmt.Sprintf("am kill %s 2>/dev/null", pkg))
		if pid = pidof(pkg); pid != "" {
			log.Warn(fmt.Sprintf("  %s is STILL running (pid %s); it may compete with SerialBridge", pkg, pid))
			log.Warn("  Reboot the bike after jailbreak finishes to clear it.")
		} else {
			log.Dim(fmt.Sprintf("  %s stopped", pkg))
		}
	}
}

var overlayAllowRe = regexp.MustCompile(`SYSTEM_ALERT_WINDOW:\s*allow\b`)

// overlayAllowed reports whether "appops get" output shows the overlay allowed.
func overlayAllowed(appopsOut string) bool {
	return overlayAllowRe.MatchString(appopsOut)
}

// checkBridgeHealth waits briefly for a SerialBridge DIAG line (bridge
// v1.5.0+) and logs a plain verdict. Older bridges don't log DIAG, so fall
// back to checking that TCP:9999 is listening. Never fails the jailbreak.
func checkBridgeHealth(connect func() (*adb.Conn, error), log Logger) {
	// Only trust lines logged from now on, so a bridge from before an update
	// can't produce a stale verdict. -1 (unknown) accepts any line.
	since := -1.0
	if conn, err := connect(); err == nil {
		out, _ := conn.Shell("date +%s")
		conn.Close()
		if ts, err := strconv.ParseFloat(strings.TrimSpace(out), 64); err == nil {
			since = ts
		}
	}

	if conn, err := connect(); err == nil {
		dev, _ := conn.Shell("settings get global freewheel_serial_dev 2>/dev/null")
		baud, _ := conn.Shell("settings get global freewheel_serial_baud 2>/dev/null")
		conn.Close()
		dev, baud = strings.TrimSpace(dev), strings.TrimSpace(baud)
		if (dev != "" && dev != "null") || (baud != "" && baud != "null") {
			log.Info(fmt.Sprintf("  Serial port override active: freewheel_serial_dev=%s freewheel_serial_baud=%s", dev, baud))
		}
	}

	log.Dim("  Waiting for SerialBridge status (up to 15s)...")
	deadline := time.Now().Add(15 * time.Second)
	for {
		if conn, err := connect(); err == nil {
			out, _ := conn.Shell("logcat -d -v epoch -s SerialBridge:I 2>/dev/null | grep ' DIAG ' | tail -n 5")
			conn.Close()
			if d, ok := latestDiag(out, since); ok {
				reportDiag(d, log)
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(3 * time.Second)
	}

	log.Dim("  No status line from SerialBridge (older bridge build?) — checking TCP:9999")
	conn, err := connect()
	if err != nil {
		log.Warn(fmt.Sprintf("  Could not reconnect to check SerialBridge: %v", err))
		return
	}
	tcpCheck, _ := conn.Shell("nc -z localhost 9999 2>/dev/null && echo ALIVE || echo DEAD")
	conn.Close()
	if strings.Contains(tcpCheck, "ALIVE") {
		log.Success("  TCP:9999 alive (SerialBridge serving sensor data)")
	} else {
		log.Dim("  TCP:9999 not yet listening (SerialBridge may need a moment)")
	}
}

var installFailureRe = regexp.MustCompile(`INSTALL_FAILED_[A-Z_]+`)

// installFailureCode returns the INSTALL_FAILED_* code in "pm install"
// output, or "" if there is none.
func installFailureCode(out string) string {
	return installFailureRe.FindString(out)
}

// installNeedsUninstall reports whether "pm install -r" failed only because
// the installed copy blocks an in-place update, so uninstalling it first and
// retrying can succeed. SHARED_USER_INCOMPATIBLE is not one of them: the new
// APK isn't signed for android.uid.system, so uninstalling the working
// SerialBridge would only lose it.
func installNeedsUninstall(out string) bool {
	switch installFailureCode(out) {
	case "INSTALL_FAILED_UPDATE_INCOMPATIBLE",
		"INSTALL_FAILED_VERSION_DOWNGRADE":
		return true
	}
	return false
}

// reportUpdateFailureState checks what a failed update left on the bike and
// tells the user plainly. If VeloLauncher is gone, the stock Android launcher
// is re-enabled so the bike still has a home screen.
func reportUpdateFailureState(connect func() (*adb.Conn, error), log Logger) {
	conn, err := connect()
	if err != nil {
		log.Error(fmt.Sprintf("  Could not reconnect to check the bike: %v", err))
		log.Warn("  Run Jailbreak again once the bike is reachable. If the screen is blank, run Restore.")
		return
	}
	defer conn.Close()
	out, err := conn.Shell("pm list packages 2>/dev/null | grep -E 'freewheel.launcher|freewheel.bridge'")
	if err != nil {
		log.Warn("  Could not check which FreeWheel apps are installed. Run Jailbreak again.")
		return
	}
	hasLauncher := strings.Contains(out, "io.freewheel.launcher")
	hasBridge := strings.Contains(out, "io.freewheel.bridge")

	if hasLauncher {
		log.Info("  VeloLauncher is still installed.")
	} else {
		log.Error("  VeloLauncher is NOT installed — re-enabling the stock Android launcher so the bike has a home screen")
		conn.Shell("pm enable com.android.launcher3 2>/dev/null")
		conn.Shell("pm enable com.android.quickstep 2>/dev/null")
		conn.Shell("input keyevent KEYCODE_HOME")
	}
	if hasBridge {
		log.Info("  SerialBridge is still installed.")
	} else {
		log.Error("  SerialBridge is NOT installed — bike sensors will not work")
	}
	if !hasLauncher || !hasBridge {
		log.Warn("  Run Jailbreak again to finish the update, or Restore to go back to stock.")
	}
}
