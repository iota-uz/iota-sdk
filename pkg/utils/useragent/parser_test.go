package useragent_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/utils/useragent"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		userAgent    string
		wantBrowser  string
		wantOS       string
		wantDevice   string
		wantIsMobile bool
		wantIcon     string
	}{
		{
			name:         "Chrome on Windows Desktop",
			userAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			wantBrowser:  "Chrome",
			wantOS:       "Windows",
			wantDevice:   "Desktop",
			wantIsMobile: false,
			wantIcon:     "desktop",
		},
		{
			name:         "Safari on macOS Desktop",
			userAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
			wantBrowser:  "Safari",
			wantOS:       "macOS",
			wantDevice:   "Desktop",
			wantIsMobile: false,
			wantIcon:     "desktop",
		},
		{
			name:         "Firefox on Linux Desktop",
			userAgent:    "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0",
			wantBrowser:  "Firefox",
			wantOS:       "Linux",
			wantDevice:   "Desktop",
			wantIsMobile: false,
			wantIcon:     "desktop",
		},
		{
			name:         "Safari on iPhone (Mobile)",
			userAgent:    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantBrowser:  "Safari",
			wantOS:       "iOS",
			wantDevice:   "Mobile",
			wantIsMobile: true,
			wantIcon:     "device-mobile",
		},
		{
			name:         "Chrome on Android Mobile",
			userAgent:    "Mozilla/5.0 (Linux; Android 13; SM-S911B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.43 Mobile Safari/537.36",
			wantBrowser:  "Chrome",
			wantOS:       "Android",
			wantDevice:   "Mobile",
			wantIsMobile: true,
			wantIcon:     "device-mobile",
		},
		{
			name:         "Safari on iPad (Tablet)",
			userAgent:    "Mozilla/5.0 (iPad; CPU OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantBrowser:  "Safari",
			wantOS:       "iOS",
			wantDevice:   "Tablet",
			wantIsMobile: false,
			wantIcon:     "device-tablet",
		},
		{
			name:         "Chrome on Android Tablet",
			userAgent:    "Mozilla/5.0 (Linux; Android 13; SM-X900) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.43 Safari/537.36",
			wantBrowser:  "Chrome",
			wantOS:       "Android",
			wantDevice:   "Mobile",
			wantIsMobile: true,
			wantIcon:     "device-mobile",
		},
		{
			name:         "Edge on Windows Desktop",
			userAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			wantBrowser:  "Edge",
			wantOS:       "Windows",
			wantDevice:   "Desktop",
			wantIsMobile: false,
			wantIcon:     "desktop",
		},
		{
			name:         "Empty user agent string",
			userAgent:    "",
			wantBrowser:  "",
			wantOS:       "",
			wantDevice:   "Unknown",
			wantIsMobile: false,
			wantIcon:     "desktop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := useragent.Parse(tt.userAgent)

			assert.Equal(t, tt.wantBrowser, result.Browser, "Browser mismatch")
			assert.Equal(t, tt.wantOS, result.OS, "OS mismatch")
			assert.Equal(t, tt.wantDevice, result.Device, "Device type mismatch")
			assert.Equal(t, tt.wantIsMobile, result.IsMobile, "IsMobile mismatch")
			assert.Equal(t, tt.wantIcon, result.Icon, "Icon mismatch")
		})
	}
}

func TestDeviceInfo_String(t *testing.T) {
	tests := []struct {
		name       string
		deviceInfo useragent.DeviceInfo
		wantOutput string
	}{
		{
			name: "Full info with browser and OS versions",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Chrome",
				BrowserVersion: "120.0.0.0",
				OS:             "Windows",
				OSVersion:      "10.0",
				Device:         "Desktop",
				IsMobile:       false,
				Icon:           "desktop",
			},
			wantOutput: "Chrome 120.0.0.0 on Windows 10.0 (Desktop)",
		},
		{
			name: "Browser version only",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Safari",
				BrowserVersion: "17.0",
				OS:             "macOS",
				OSVersion:      "",
				Device:         "Desktop",
				IsMobile:       false,
				Icon:           "desktop",
			},
			wantOutput: "Safari 17.0 on macOS (Desktop)",
		},
		{
			name: "OS version only",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Firefox",
				BrowserVersion: "",
				OS:             "Linux",
				OSVersion:      "6.5.0",
				Device:         "Desktop",
				IsMobile:       false,
				Icon:           "desktop",
			},
			wantOutput: "Firefox on Linux 6.5.0 (Desktop)",
		},
		{
			name: "No versions",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Chrome",
				BrowserVersion: "",
				OS:             "Android",
				OSVersion:      "",
				Device:         "Mobile",
				IsMobile:       true,
				Icon:           "device-mobile",
			},
			wantOutput: "Chrome on Android (Mobile)",
		},
		{
			name: "Mobile device",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Safari",
				BrowserVersion: "17.0",
				OS:             "iOS",
				OSVersion:      "17.1.1",
				Device:         "Mobile",
				IsMobile:       true,
				Icon:           "device-mobile",
			},
			wantOutput: "Safari 17.0 on iOS 17.1.1 (Mobile)",
		},
		{
			name: "Tablet device",
			deviceInfo: useragent.DeviceInfo{
				Browser:        "Safari",
				BrowserVersion: "17.0",
				OS:             "iOS",
				OSVersion:      "17.1.1",
				Device:         "Tablet",
				IsMobile:       false,
				Icon:           "device-tablet",
			},
			wantOutput: "Safari 17.0 on iOS 17.1.1 (Tablet)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.deviceInfo.String()
			assert.Equal(t, tt.wantOutput, result)
		})
	}
}

func TestParse_RealWorldUserAgents(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
	}{
		{
			name:      "Chrome 120 Windows 11",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		{
			name:      "iPhone 15 Pro Safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		},
		{
			name:      "Samsung Galaxy S23 Chrome",
			userAgent: "Mozilla/5.0 (Linux; Android 13; SM-S911B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.43 Mobile Safari/537.36",
		},
		{
			name:      "iPad Pro Safari",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		},
		{
			name:      "macOS Sonoma Safari",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		},
		{
			name:      "Ubuntu Firefox",
			userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := useragent.Parse(tt.userAgent)

			// Verify basic fields are populated
			assert.NotEmpty(t, result.Browser, "Browser should not be empty")
			assert.NotEmpty(t, result.OS, "OS should not be empty")
			assert.NotEmpty(t, result.Device, "Device should not be empty")
			assert.NotEmpty(t, result.Icon, "Icon should not be empty")

			// Verify icon mapping is correct
			if result.IsMobile {
				assert.Equal(t, "device-mobile", result.Icon, "Mobile devices should have device-mobile icon")
			} else if result.Device == "Tablet" {
				assert.Equal(t, "device-tablet", result.Icon, "Tablets should have device-tablet icon")
			} else if result.Device == "Desktop" {
				assert.Equal(t, "desktop", result.Icon, "Desktops should have desktop icon")
			}

			// Verify String() method returns a valid string
			stringOutput := result.String()
			assert.NotEmpty(t, stringOutput, "String() should return a non-empty string")
		})
	}
}
