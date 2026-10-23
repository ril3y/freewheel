package device

import "testing"

func TestInstallNeedsUninstall(t *testing.T) {
	cases := []struct {
		out  string
		want bool
	}{
		{"Success", false},
		{"", false},
		{"Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE: Package io.freewheel.launcher signatures do not match the previously installed version; ignoring!]", true},
		{"Failure [INSTALL_FAILED_SHARED_USER_INCOMPATIBLE: Package couldn't be installed in /data/app/x: Package io.freewheel.bridge has no signatures that match those in shared user android.uid.system; ignoring!]", false},
		{"Failure [INSTALL_FAILED_VERSION_DOWNGRADE]", true},
		{"Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]", false},
		{"Failure [INSTALL_PARSE_FAILED_NO_CERTIFICATES: ...]", false},
	}
	for _, c := range cases {
		if got := installNeedsUninstall(c.out); got != c.want {
			t.Errorf("installNeedsUninstall(%q) = %v, want %v", c.out, got, c.want)
		}
	}
}

func TestListsPackage(t *testing.T) {
	out := "package:com.nautilus.bowflex.usb.helper\npackage:com.nautilus.nautiluslauncher\r\n"
	if listsPackage(out, "com.nautilus.bowflex.usb") {
		t.Error("substring match counted as installed")
	}
	if !listsPackage(out, "com.nautilus.nautiluslauncher") {
		t.Error("exact match with CRLF not found")
	}
	if listsPackage("", "com.nautilus.bowflex.usb") {
		t.Error("empty output counted as installed")
	}
}
