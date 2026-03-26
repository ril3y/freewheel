# FreeWheel

**Custom firmware launcher for the Bowflex Velocore bike.**

FreeWheel replaces the stock JRNY app with VeloLauncher — an open-source fitness platform that lets you ride with Netflix, play retro games, and track your workouts without a subscription.

## What it does

1. **Scans** your network for a Bowflex Velocore bike with ADB enabled
2. **Pre-checks** device compatibility (Android 9, Rockchip board, JRNY version)
3. **Jailbreaks** — disables JRNY, installs VeloLauncher + SerialBridge
4. **Restores** — one-click revert to stock Nautilus configuration

## Requirements

- Bowflex Velocore C9 or C10 bike
- WiFi network (bike and PC on same network)
- ADB debugging enabled on the bike (see setup guide)
- Windows PC

## Quick Start

1. Download `freewheel.exe` from [Releases](https://github.com/ril3y/bowflex-tool/releases)
2. Enable ADB on your bike (Settings → Developer Options → USB Debugging → ON)
3. Run `freewheel.exe`
4. Click **Scan Network** to find your bike
5. Click **Pre-Check** to verify compatibility
6. Click **Jailbreak!** to install VeloLauncher

## What gets installed

| App | Purpose |
|-----|---------|
| **VeloLauncher** | Custom home screen with workout picker, ride tracking, media overlay, OTA updates |
| **SerialBridge** | Bridges the bike's UCB hardware (sensors, resistance) to Android apps |

## Features after jailbreak

- **Free ride** or **structured workouts** with real-time power, RPM, heart rate
- **Netflix/YouTube overlay** — watch media while riding with floating stats
- **Auto-pause** when you stop pedaling
- **Ride history** with stats tracking
- **OTA updates** — the launcher checks for updates automatically
- **Third-party fitness apps** via the WorkoutSession API (see [Developer Guide](https://github.com/ril3y/velo-platform/blob/main/DEVELOPER.md))

## Restore to stock

Click **Restore Stock** in FreeWheel to revert everything. The bike will return to the original JRNY experience.

## Building from source

```bash
go build -o freewheel.exe ./cmd/freewheel
```

Requires Go 1.25+ and the Fyne GUI toolkit dependencies.

## Related projects

- [velo-platform](https://github.com/ril3y/velo-platform) — VeloLauncher, SerialBridge, and fitness SDK
- [bike-arcade](https://github.com/ril3y/bike-arcade) — Retro arcade games controlled by pedaling (separate app)

## License

MIT
