package unlock

import "testing"

// TestAvailabilityToErrorMapsNoActiveWindowAsFallbackable は、setupに一時的な
// オーナーウィンドウ不在が「この環境では使えない」という恒久判定に化けないこと
// を確認する（監査 F-19）。NoActiveWindowはErrorUnsupportedではなく
// ErrorUnavailable（fallback許可）へ写像されなければならない。
func TestAvailabilityToErrorMapsNoActiveWindowAsFallbackable(t *testing.T) {
	err := AvailabilityToError(AvailabilityNoActiveWindow)
	kind, ok := ProviderErrorKindOf(err)
	if !ok || kind != ErrorUnavailable {
		t.Fatalf("kind = %v (ok=%t), want ErrorUnavailable", kind, ok)
	}
	if !IsFallbackError(err) {
		t.Fatal("NoActiveWindow はfallback可能でなければならない")
	}
}

func TestAvailabilityToErrorMapping(t *testing.T) {
	cases := map[Availability]ErrorKind{
		AvailabilityAvailable:        "",
		AvailabilityNotConfigured:    ErrorNotConfigured,
		AvailabilityDeviceNotPresent: ErrorUnsupported,
		AvailabilityDisabledByPolicy: ErrorUnsupported,
		AvailabilityUnsupported:      ErrorUnsupported,
		AvailabilityDeviceBusy:       ErrorDeviceBusy,
		AvailabilityTimeout:          ErrorTimeout,
		AvailabilityNoActiveWindow:   ErrorUnavailable,
		AvailabilityUnavailable:      ErrorUnavailable,
	}
	for availability, want := range cases {
		t.Run(string(availability), func(t *testing.T) {
			err := AvailabilityToError(availability)
			if want == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			kind, ok := ProviderErrorKindOf(err)
			if !ok || kind != want {
				t.Fatalf("kind = %v (ok=%t), want %q", kind, ok, want)
			}
		})
	}
}
