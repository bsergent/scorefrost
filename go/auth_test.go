package main

import (
	"net/http"
	"testing"
)

func TestGetIPAddress(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "Standard RemoteAddr",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.195",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.196",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.196",
		},
		{
			name: "Multiple X-Forwarded-For IPs",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.195", // Should get the first one
		},
		{
			name:       "IPv6 RemoteAddr",
			remoteAddr: "[2001:db8::1]:12345",
			expectedIP: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Set headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Set RemoteAddr
			req.RemoteAddr = tt.remoteAddr

			ip := getIPAddress(req)

			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test when no user ID is set
	_, found := GetUserID(req)
	if found {
		t.Error("Expected no user ID, but found one")
	}

	// Test when no display name is set
	_, found = GetDisplayName(req)
	if found {
		t.Error("Expected no display name, but found one")
	}

	// Test when no friend code is set
	_, found = GetFriendCode(req)
	if found {
		t.Error("Expected no friend code, but found one")
	}
}
