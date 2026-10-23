package ui

// disclaimerText is shown before a jailbreak runs. Keep in sync with the
// "Use at your own risk" section of README.md.
const disclaimerText = "USE AT YOUR OWN RISK.\n\n" +
	"FreeWheel / VeloLauncher is an unofficial community project, not affiliated with " +
	"or endorsed by Bowflex, Nautilus or JRNY.\n\n" +
	"The maintainer no longer owns a Velocore bike, so releases from FreeWheel v2.1.0 " +
	"(installer) / VeloLauncher v1.5.0 onward have NOT been tested on real hardware.\n\n" +
	"Modifying your bike may void your warranty and could leave it in a state that " +
	"requires a factory reset.\n\n" +
	"Provided AS IS, without warranty of any kind (see LICENSE)."

// disclaimerOneLine is logged at startup.
const disclaimerOneLine = "USE AT YOUR OWN RISK: unofficial, not affiliated with Bowflex/Nautilus/JRNY; untested on real hardware since v2.1.0. Provided AS IS."

// noJRNYOverrideText is shown when Pre-Check finds compatible Nautilus
// hardware without JRNY, before the user may jailbreak it anyway.
const noJRNYOverrideText = "This bike runs Nautilus firmware on compatible hardware, but JRNY " +
	"and the other Nautilus apps are not installed (usually because the bike was " +
	"factory reset and never finished the Nautilus setup).\n\n" +
	"FreeWheel can still jailbreak it, but Restore Stock cannot bring JRNY back " +
	"afterwards: it only re-enables apps that are already on the bike. To go back " +
	"to stock you would need to factory reset the bike and run the Nautilus setup, " +
	"which may update the firmware.\n\n" +
	"Allow the jailbreak anyway?"
