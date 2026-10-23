<p align="center">
  <img src="assets/logo.png" alt="FreeWheel" width="120">
</p>

# FreeWheel

**Custom firmware launcher for the Bowflex Velocore bike.**

FreeWheel replaces the stock JRNY app with [VeloLauncher](https://github.com/ril3y/velo-platform) — an open-source fitness platform that lets you ride with Netflix, track workouts, and use the bike without a subscription.

No command-line tools, Android SDK, or USB cable required. Just download, run, and follow 3 steps.

> [!WARNING]
> **Read the [Use at your own risk](#use-at-your-own-risk) section before jailbreaking.** Current releases have not been tested on real hardware.

## Use at your own risk

**USE AT YOUR OWN RISK.** FreeWheel/VeloLauncher is an unofficial community project, not affiliated with or endorsed by Bowflex/Nautilus/JRNY.

The maintainer no longer owns a Velocore bike, so releases from **v2.1.0 (installer) / v1.5.0 (launcher)** onward have **NOT been tested on real hardware**. They are built from changes that could only be checked by reading code and running unit tests.

Modifying your bike may void your warranty and could leave it in a state requiring a factory reset. Provided AS IS, without warranty of any kind (see [LICENSE](LICENSE)).

FreeWheel asks you to accept this before it jailbreaks. **Restore Stock** is always available without it.

## Setup

### Step 1: Enable ADB on the bike

1. Power on your Velocore bike and wait for the JRNY boot screen
2. **Tap the top-right corner of the screen 9 times rapidly** — this opens the hidden "Advanced User Actions" menu
3. Open the **Utility App**
4. The Utility App enables ADB over WiFi automatically (port 5555)
5. Leave the Utility App running — do NOT close it

### Step 2: Run FreeWheel on your PC

1. Download `freewheel.exe` from [Releases](https://github.com/ril3y/freewheel/releases) (optionally check it against the release's `SHA256SUMS`)
2. Run it (no installation needed)
3. Click **Scan Network** — FreeWheel will find your bike on the local network
4. If scan doesn't find it, enter the bike's IP address manually

### Step 3: Jailbreak

1. Click **Pre-Check** — verifies your bike is compatible (Android 9, Rockchip board, correct JRNY version)
2. Click **Jailbreak!** and accept the [disclaimer](#use-at-your-own-risk)
3. **On the bike**: tap **Allow** when the USB debugging authorization prompt appears
4. Wait for the 9-step process to complete (~2 minutes)
5. VeloLauncher becomes the home screen. The last step checks the bike's sensor link and prints the result (see [Sensors show zero](#sensors-show-zero) if it reports a problem)

If anything fails on a fresh jailbreak, FreeWheel automatically restores the bike to stock. Running Jailbreak on a bike that is already jailbroken updates both apps in place.

**Updating from FreeWheel v2.0.1 or earlier:** run Jailbreak once from the new FreeWheel. The launcher's signing key changed, so its built-in updater can't upgrade the old install; FreeWheel reinstalls it (this clears VeloLauncher's app data on the bike, including local ride history).

## What gets installed

| App | Purpose |
|-----|---------|
| **VeloLauncher** | Custom home screen — workouts, free ride, media overlay, ride history, settings, OTA updates |
| **SerialBridge** | Platform-signed service that bridges the bike's hardware sensors to Android apps |

Both come from the [velo-platform release](https://github.com/ril3y/velo-platform/releases) pinned in [`VELO_PLATFORM_VERSION`](VELO_PLATFORM_VERSION). FreeWheel never installs accounts or app data.

## Features

- **Structured workouts** with segments, difficulty scaling, and auto-pause when you stop pedaling
- **Free ride** mode — just ride and track stats
- **Netflix/YouTube overlay** — watch media with floating ride stats on top
- **5-4-3-2-1 countdown** before workouts start
- **Ride summary** after every workout (calories, power, RPM, distance, heart rate)
- **Ride history** with streak tracking
- **OTA updates** — the launcher checks for updates from GitHub automatically
- **Third-party fitness app support** via the WorkoutSession API

## Restore to stock

Open FreeWheel, click **Scan Network** (or enter the IP), then click **Restore Stock**. Everything reverts to the original JRNY experience in ~1 minute. Google services and WiFi stay enabled, and ADB over WiFi stays on so you can reconnect later.

## Troubleshooting

**Can't find the bike on the network?**
- Make sure your PC and bike are on the same WiFi network
- Make sure the Utility App is running on the bike
- Try entering the bike's IP address manually (check your router's connected devices)
- Bike on a different subnet or VLAN (e.g. an IoT network)? Scan only covers the PC's own networks. Type that subnet in the IP box (e.g. `192.168.10.0`) and click **Scan Network**, and make sure your firewall lets the PC reach the bike on TCP port 5555

**"Pre-Check failed"?**
- Only Velocore C9/C10 bikes are supported (Android 9, Rockchip board)
- JRNY version 2.25.1 is tested; other versions may work but aren't guaranteed

**Bike stuck after jailbreak?**
- Swipe up from the bottom edge of the screen to get home (invisible gesture zone)
- If completely stuck, use FreeWheel's Restore Stock feature from your PC

### Sensors show zero

- **Start a ride first.** The bike only streams RPM/power/resistance once a ride is running, so zeros on the home screen are normal.
- In VeloLauncher open **Settings > Diagnostics** and check the status line (is the serial port open, are frames arriving?) and the **FW** line (firmware state).
- Re-run **Jailbreak** (it updates in place). Its last step reports whether the sensors are talking, getting no data, or getting corrupted data because another app shares the serial port.
- If values stay at zero during a ride, collect logs with [ADB platform-tools](https://developer.android.com/tools/releases/platform-tools) from a PC on the same WiFi and [open an issue](https://github.com/ril3y/freewheel/issues) with them:
  ```
  adb connect <bike-ip>:5555
  adb logcat -d -s SerialBridge:* > bridge.txt
  adb shell ps -A | grep -i nautilus
  ```
  The last command should print nothing. If it lists `com.nautilus.nautiluslauncher` or `com.nautilus.bowflex.usb`, a stock app is competing for the serial port.

### Apps are signed into someone else's account

FreeWheel never ships accounts or app data. If Netflix, YouTube or Google show another person's account, the bike was most likely used or refurbished and still has the previous owner's data.
- On the bike: **Settings > Accounts**, remove the account(s)
- Or clear the app's data over ADB, for example:
  ```
  adb shell pm clear com.netflix.mediaclient
  adb shell pm clear com.google.android.youtube
  ```
- Or factory reset the bike, then run FreeWheel again

### Bike turns back on after shutdown

Known issue ([#1](https://github.com/ril3y/freewheel/issues/1)), still under investigation. For now use sleep, or unplug the bike to power it off fully. If you can, please report on the issue whether the stock JRNY software does the same on your bike.

### Overlay / floating home button missing

The overlay needs the "display over other apps" permission. FreeWheel grants it during the jailbreak and warns if that didn't work. To grant it manually:
```
adb connect <bike-ip>:5555
adb shell appops set io.freewheel.launcher SYSTEM_ALERT_WINDOW allow
```

## Building from source

Releases are built by GitHub Actions ([`.github/workflows`](.github/workflows)) with [`build/ci-build.sh`](build/ci-build.sh), which downloads the APKs from the velo-platform release and builds `freewheel.exe`.

Locally on Windows, run `build\build.bat` (needs Go 1.25+, a MinGW gcc for Fyne/cgo, and the [GitHub CLI](https://cli.github.com/) to fetch the APKs). Or by hand:

```bash
gh release download "$(cat VELO_PLATFORM_VERSION)" -R ril3y/velo-platform -p velolauncher.apk -p serialbridge.apk -D assets
go build -ldflags "-H windowsgui -X main.version=dev" -o freewheel.exe ./cmd/freewheel
```

## Related

- [velo-platform](https://github.com/ril3y/velo-platform) — VeloLauncher source, SerialBridge, fitness SDK
- [Developer Guide](https://github.com/ril3y/velo-platform/blob/main/DEVELOPER.md) — Build fitness apps for the platform

## License

[MIT](LICENSE). Provided AS IS, without warranty of any kind. See [Use at your own risk](#use-at-your-own-risk).
