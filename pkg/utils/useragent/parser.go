package useragent

import (
	"fmt"

	"github.com/mileusna/useragent"
)

// DeviceInfo contains parsed information from a user agent string
type DeviceInfo struct {
	Browser        string
	BrowserVersion string
	OS             string
	OSVersion      string
	Device         string
	IsMobile       bool
	Icon           string
}

// Parse extracts device information from a user agent string
func Parse(userAgentString string) DeviceInfo {
	ua := useragent.Parse(userAgentString)

	info := DeviceInfo{
		Browser:        ua.Name,
		BrowserVersion: ua.Version,
		OS:             ua.OS,
		OSVersion:      ua.OSVersion,
		IsMobile:       ua.Mobile,
	}

	// Determine device type and icon
	if ua.Mobile {
		info.Device = "Mobile"
		info.Icon = "device-mobile"
	} else if ua.Tablet {
		info.Device = "Tablet"
		info.Icon = "device-tablet"
	} else if ua.Desktop {
		info.Device = "Desktop"
		info.Icon = "desktop"
	} else {
		// Fallback for unknown device types
		info.Device = "Unknown"
		info.Icon = "desktop"
	}

	return info
}

// String returns a human-readable representation of the device info
func (d DeviceInfo) String() string {
	if d.BrowserVersion != "" && d.OSVersion != "" {
		return fmt.Sprintf("%s %s on %s %s (%s)",
			d.Browser, d.BrowserVersion, d.OS, d.OSVersion, d.Device)
	} else if d.BrowserVersion != "" {
		return fmt.Sprintf("%s %s on %s (%s)",
			d.Browser, d.BrowserVersion, d.OS, d.Device)
	} else if d.OSVersion != "" {
		return fmt.Sprintf("%s on %s %s (%s)",
			d.Browser, d.OS, d.OSVersion, d.Device)
	}
	return fmt.Sprintf("%s on %s (%s)", d.Browser, d.OS, d.Device)
}
