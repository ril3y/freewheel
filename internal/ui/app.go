package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ril3y/bowflex-tool/assets"
	"github.com/ril3y/bowflex-tool/internal/device"
)

// Run creates and runs the FreeWheel application. version and gitHash are
// stamped in at build time (see build/ci-build.sh).
func Run(version, gitHash string) {
	// Wire up the FindAPK function for device operations
	device.FindAPKFunc = assets.FindAPK

	os.Setenv("FYNE_THEME", "dark")

	a := app.NewWithID("com.battlewithbytes.freewheel")
	a.Settings().SetTheme(theme.DarkTheme())
	versionLabel := version
	if versionLabel != "dev" && !strings.HasPrefix(versionLabel, "v") {
		versionLabel = "v" + versionLabel
	}
	w := a.NewWindow("FreeWheel " + versionLabel)
	w.Resize(fyne.NewSize(520, 600))
	w.SetFixedSize(true)

	log := NewLogger()

	// --- Log controls ---
	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(log.CopyAll())
	})
	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		log.Clear()
	})

	// --- Device list ---
	var foundDevices []string
	selectedDevice := ""
	deviceList := widget.NewList(
		func() int { return len(foundDevices) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(foundDevices) {
				obj.(*widget.Label).SetText(fmt.Sprintf("  %s  (port 5555)", foundDevices[id]))
			}
		},
	)
	deviceList.OnSelected = func(id widget.ListItemID) {
		if id < len(foundDevices) {
			selectedDevice = foundDevices[id]
		}
	}

	// --- Manual IP entry ---
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("192.168.1.xxx")

	// --- Helper: resolve target IP ---
	getIP := func() string {
		ip := ipEntry.Text
		if ip == "" {
			ip = selectedDevice
		}
		if strings.Contains(ip, ":") {
			ip = strings.Split(ip, ":")[0]
		}
		return ip
	}

	// --- Buttons ---
	// preCheckedIP is the IP that last passed Pre-Check ("" = none). Like the
	// other UI state here it is only touched on the Fyne goroutine.
	var scanBtn, precheckBtn, jailbreakBtn, restoreBtn *widget.Button
	preCheckedIP := ""

	scanBtn = widget.NewButtonWithIcon("Scan Network", theme.SearchIcon(), func() {
		scanBtn.Disable()
		log.Info("Scanning network for Bowflex bikes (port 5555)...")
		go func() {
			devices := device.ScanNetwork(log)
			if len(devices) > 0 {
				log.Dim("Checking device status...")
				if device.IsJailbroken(devices[0], log) {
					log.Info("Device is jailbroken -- Restore Stock available")
				}
			} else {
				log.Warn("No devices found. Enter IP manually or check instructions above.")
			}
			fyne.Do(func() {
				foundDevices = devices
				deviceList.Refresh()
				if len(devices) > 0 {
					deviceList.Select(0)
				}
				scanBtn.Enable()
			})
		}()
	})

	precheckBtn = widget.NewButtonWithIcon("Pre-Check", theme.InfoIcon(), func() {
		ip := getIP()
		if ip == "" {
			log.Error("No device selected and no IP entered!")
			return
		}
		precheckBtn.Disable()
		scanBtn.Disable()
		go func() {
			passed := device.PreflightCheck(ip, log)
			fyne.Do(func() {
				preCheckedIP = ""
				if passed {
					preCheckedIP = ip
				}
				precheckBtn.Enable()
				scanBtn.Enable()
				if passed {
					jailbreakBtn.Enable()
					log.Success("Pre-check passed! Jailbreak button is now enabled.")
				} else {
					jailbreakBtn.Disable()
					log.Error("Pre-check FAILED. Copy the output above and send to FreeWheel devs.")
					log.Warn("Use the copy button (top-right of log) to copy the full output.")
				}
			})
		}()
	})

	jailbreakBtn = widget.NewButtonWithIcon("  Jailbreak!  ", theme.ConfirmIcon(), func() {
		ip := getIP()
		if ip == "" {
			log.Error("No device selected and no IP entered!")
			return
		}
		if preCheckedIP == "" {
			log.Error("Run Pre-Check first!")
			return
		}
		if ip != preCheckedIP {
			log.Error(fmt.Sprintf("Target changed to %s since Pre-Check passed for %s. Run Pre-Check again.", ip, preCheckedIP))
			preCheckedIP = ""
			jailbreakBtn.Disable()
			return
		}
		// The user must accept the disclaimer before anything is changed on
		// the bike. Restore Stock is deliberately not gated by this.
		disclaimer := widget.NewLabel(disclaimerText)
		disclaimer.Wrapping = fyne.TextWrapWord
		confirm := dialog.NewCustomConfirm("Before you jailbreak", "I understand, continue", "Cancel",
			container.NewVScroll(disclaimer), func(accepted bool) {
				if !accepted {
					log.Dim("Jailbreak cancelled.")
					return
				}
				log.Dim("Disclaimer accepted.")
				jailbreakBtn.Disable()
				precheckBtn.Disable()
				scanBtn.Disable()
				restoreBtn.Disable()
				go func() {
					device.RunJailbreak(ip, log)
					fyne.Do(func() {
						jailbreakBtn.Disable()
						preCheckedIP = ""
						precheckBtn.Enable()
						scanBtn.Enable()
						restoreBtn.Enable()
					})
				}()
			}, w)
		confirm.Resize(fyne.NewSize(460, 420))
		confirm.Show()
	})
	jailbreakBtn.Importance = widget.HighImportance
	jailbreakBtn.Disable() // disabled until pre-check passes

	restoreBtn = widget.NewButtonWithIcon("Restore Stock", theme.MediaReplayIcon(), func() {
		ip := getIP()
		if ip == "" {
			log.Error("No device selected and no IP entered!")
			return
		}
		restoreBtn.Disable()
		jailbreakBtn.Disable()
		precheckBtn.Disable()
		scanBtn.Disable()
		go func() {
			device.RunRestore(ip, log)
			fyne.Do(func() {
				restoreBtn.Enable()
				jailbreakBtn.Disable()
				preCheckedIP = ""
				precheckBtn.Enable()
				scanBtn.Enable()
			})
		}()
	})
	// Always shown: Restore must be reachable with a typed-in IP when Scan
	// can't find the bike, and it isn't gated by Pre-Check or the disclaimer.
	restoreBtn.Importance = widget.DangerImportance

	// --- Logo ---
	logoRes := fyne.NewStaticResource("logo.png", assets.EmbeddedLogo)
	logoImg := canvas.NewImageFromResource(logoRes)
	logoImg.FillMode = canvas.ImageFillContain
	logoImg.SetMinSize(fyne.NewSize(36, 36))
	w.SetIcon(logoRes)

	// --- Header ---
	titleText := canvas.NewText("FreeWheel", ColorSuccess)
	titleText.TextSize = 22
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText("Bowflex VeloCore Jailbreak", color.NRGBA{R: 140, G: 140, B: 160, A: 255})
	subtitleText.TextSize = 11

	titleBlock := container.NewVBox(titleText, subtitleText)
	headerRow := container.NewHBox(logoImg, container.NewPadded(titleBlock))

	// --- Quick-start hints ---
	hint1 := canvas.NewText("1. On bike: tap top-right corner 9x, open Utility App", color.NRGBA{R: 170, G: 170, B: 190, A: 255})
	hint1.TextSize = 11
	hint2 := canvas.NewText("2. Here: Scan or enter IP, Pre-Check, then Jailbreak", color.NRGBA{R: 170, G: 170, B: 190, A: 255})
	hint2.TextSize = 11
	hint3 := canvas.NewText("3. On bike: tap Allow if USB debugging prompt appears", color.NRGBA{R: 170, G: 170, B: 190, A: 255})
	hint3.TextSize = 11
	hintsBox := container.NewVBox(hint1, hint2, hint3)

	// --- Accent line ---
	accentLine := canvas.NewRectangle(color.NRGBA{R: 0, G: 255, B: 136, A: 60})
	accentLine.SetMinSize(fyne.NewSize(0, 2))

	// --- Device section ---
	deviceListSized := container.NewGridWrap(fyne.NewSize(456, 64), deviceList)
	ipRow := container.NewBorder(nil, nil,
		widget.NewLabel("IP:"), scanBtn,
		ipEntry,
	)
	deviceSection := container.NewVBox(deviceListSized, ipRow)
	deviceCard := widget.NewCard("", "Target Device", deviceSection)

	// --- Action buttons ---
	buttonRow := container.NewHBox(
		layout.NewSpacer(),
		restoreBtn,
		precheckBtn,
		jailbreakBtn,
	)

	// --- Top panel ---
	topPanel := container.NewVBox(
		container.NewPadded(headerRow),
		accentLine,
		container.NewPadded(hintsBox),
		container.NewPadded(deviceCard),
		container.NewPadded(buttonRow),
	)

	// --- Log panel ---
	logTitle := canvas.NewText("Output", color.NRGBA{R: 180, G: 180, B: 200, A: 255})
	logTitle.TextSize = 12
	logTitle.TextStyle = fyne.TextStyle{Bold: true}
	logToolbar := container.NewHBox(logTitle, layout.NewSpacer(), copyBtn, clearBtn)

	logBg := canvas.NewRectangle(color.NRGBA{R: 18, G: 18, B: 24, A: 255})
	logArea := container.NewStack(logBg, log.Widget())
	logPanel := container.NewBorder(logToolbar, nil, nil, nil, logArea)

	// --- Main layout ---
	w.SetContent(container.NewPadded(
		container.NewBorder(topPanel, nil, nil, nil, logPanel),
	))

	go func() {
		time.Sleep(300 * time.Millisecond)
		log.Info(fmt.Sprintf("FreeWheel %s (%s) ready. Native ADB -- no external tools needed.", versionLabel, gitHash))
		log.Warn(disclaimerOneLine)
	}()

	w.ShowAndRun()
}
