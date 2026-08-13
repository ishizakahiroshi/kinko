//go:build windows

package unlock

import (
	"context"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	consentVerified                 = 0
	consentDeviceNotPresent         = 1
	consentNotConfigured            = 2
	consentDisabledByPolicy         = 3
	consentDeviceBusy               = 4
	consentRetriesExhausted         = 5
	consentCanceled                 = 6
	consentPending                  = 0
	consentCompleted                = 1
	consentCanceledStatus           = 2
	consentErrorStatus              = 3
	roInitMultithreaded             = 1
	gaRoot                          = 2
	windowsHelloMinimumBuild uint32 = 22000
)

var (
	combase                    = syscall.NewLazyDLL("combase.dll")
	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoUninitialize         = combase.NewProc("RoUninitialize")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	ntdll                      = syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion          = ntdll.NewProc("RtlGetVersion")
	user32                     = syscall.NewLazyDLL("user32.dll")
	procIsWindow               = user32.NewProc("IsWindow")
	procIsWindowVisible        = user32.NewProc("IsWindowVisible")
	procGetAncestor            = user32.NewProc("GetAncestor")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow       = kernel32.NewProc("GetConsoleWindow")
)

type windowsGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	iidUserConsentVerifierInterop = windowsGUID{
		Data1: 0x39e050c3, Data2: 0x4e74, Data3: 0x441a,
		Data4: [8]byte{0x8d, 0xc0, 0xb8, 0x11, 0x04, 0xdf, 0x94, 0x9c},
	}
	iidUserConsentVerifierStatics = windowsGUID{
		Data1: 0xaf4f3f91, Data2: 0x564c, Data3: 0x4ddc,
		Data4: [8]byte{0xb8, 0xb5, 0x97, 0x34, 0x47, 0x62, 0x7c, 0x65},
	}
	iidAsyncOperationUserConsentVerificationResult = windowsGUID{
		Data1: 0xfd596ffd, Data2: 0x2318, Data3: 0x558f,
		Data4: [8]byte{0x9d, 0xbe, 0xd2, 0x1d, 0xf4, 0x37, 0x64, 0xa5},
	}
	iidAsyncInfo = windowsGUID{
		Data1: 0x00000036, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
)

type rtlOSVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
}

func windowsBuildNumber() (uint32, error) {
	info := rtlOSVersionInfoEx{
		OSVersionInfoSize: uint32(unsafe.Sizeof(rtlOSVersionInfoEx{})),
	}
	ret, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if int32(ret) != 0 {
		return 0, NewProviderError(ErrorUnsupported)
	}
	return info.BuildNumber, nil
}

func windowsBuildSupported(build uint32) bool {
	return build >= windowsHelloMinimumBuild
}

func windowsBuildGate(build uint32, next func() error) error {
	if !windowsBuildSupported(build) {
		return NewProviderError(ErrorUnsupported)
	}
	return next()
}

type windowsConsentVerifier struct {
	buildNumber func() (uint32, error)
}

func (v windowsConsentVerifier) supported(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	getBuildNumber := v.buildNumber
	if getBuildNumber == nil {
		getBuildNumber = windowsBuildNumber
	}
	build, err := getBuildNumber()
	if err != nil || !windowsBuildSupported(build) {
		return NewProviderError(ErrorUnsupported)
	}
	return nil
}

// availability calls UserConsentVerifier.CheckAvailabilityAsync and keeps the
// result separate from the later interactive request. A result of Available
// still requires a valid owner HWND before this adapter can be used by a
// desktop console invocation.
func (v windowsConsentVerifier) availability(ctx context.Context) (Availability, error) {
	if err := v.supported(ctx); err != nil {
		return AvailabilityUnsupported, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ctx.Err(); err != nil {
		return AvailabilityTimeout, providerErrorForContext(err)
	}
	initialized, ok := initializeCOM()
	if !ok {
		return AvailabilityUnsupported, NewProviderError(ErrorUnsupported)
	}
	if initialized {
		defer procRoUninitialize.Call()
	}

	class, err := syscall.UTF16PtrFromString("Windows.Security.Credentials.UI.UserConsentVerifier")
	if err != nil {
		return AvailabilityUnavailable, NewProviderError(ErrorUnavailable)
	}
	var classID uintptr
	ret, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(class)),
		uintptr(len([]rune("Windows.Security.Credentials.UI.UserConsentVerifier"))),
		uintptr(unsafe.Pointer(&classID)),
	)
	if ret != 0 || classID == 0 {
		return AvailabilityUnsupported, NewProviderError(ErrorUnsupported)
	}
	defer procWindowsDeleteString.Call(classID)

	var statics unsafe.Pointer
	ret, _, _ = procRoGetActivationFactory.Call(
		classID,
		uintptr(unsafe.Pointer(&iidUserConsentVerifierStatics)),
		uintptr(unsafe.Pointer(&statics)),
	)
	if ret != 0 || statics == nil {
		return AvailabilityUnsupported, NewProviderError(ErrorUnsupported)
	}
	defer comRelease(statics)

	var async unsafe.Pointer
	ret, _ = comCall(statics, 6, uintptr(unsafe.Pointer(&async)))
	if int32(ret) != 0 || async == nil {
		return AvailabilityUnavailable, NewProviderError(ErrorUnavailable)
	}
	defer comRelease(async)

	result, err := waitForAsyncResult(ctx, async)
	if err != nil {
		return availabilityForProviderError(err), err
	}
	availability := consentAvailability(result)
	if availability == AvailabilityAvailable && activeWindow() == 0 {
		return AvailabilityNoActiveWindow, nil
	}
	return availability, nil
}

func (v windowsConsentVerifier) verify(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	availability, err := v.availability(ctx)
	if err != nil {
		return err
	}
	if availability != AvailabilityAvailable {
		return consentAvailabilityError(availability)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	initialized, ok := initializeCOM()
	if !ok {
		return NewProviderError(ErrorUnsupported)
	}
	if initialized {
		defer procRoUninitialize.Call()
	}

	class, err := syscall.UTF16PtrFromString("Windows.Security.Credentials.UI.UserConsentVerifier")
	if err != nil {
		return NewProviderError(ErrorUnavailable)
	}
	classID := uintptr(0)
	ret, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(class)),
		uintptr(len([]rune("Windows.Security.Credentials.UI.UserConsentVerifier"))),
		uintptr(unsafe.Pointer(&classID)),
	)
	if ret != 0 || classID == 0 {
		return NewProviderError(ErrorUnsupported)
	}
	defer procWindowsDeleteString.Call(classID)

	var interop unsafe.Pointer
	ret, _, _ = procRoGetActivationFactory.Call(
		classID,
		uintptr(unsafe.Pointer(&iidUserConsentVerifierInterop)),
		uintptr(unsafe.Pointer(&interop)),
	)
	if ret != 0 || interop == nil {
		return NewProviderError(ErrorUnsupported)
	}
	defer comRelease(interop)

	message, err := syscall.UTF16PtrFromString("kinko の保管庫を開くには端末の認証が必要です")
	if err != nil {
		return NewProviderError(ErrorUnavailable)
	}
	var messageID uintptr
	ret, _, _ = procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(message)),
		uintptr(len([]rune("kinko の保管庫を開くには端末の認証が必要です"))),
		uintptr(unsafe.Pointer(&messageID)),
	)
	if ret != 0 || messageID == 0 {
		return NewProviderError(ErrorUnavailable)
	}
	defer procWindowsDeleteString.Call(messageID)

	return requestWithActiveWindow(ctx, activeWindow, func(hwnd uintptr) error {
		var async unsafe.Pointer
		ret, _ = comCall(
			interop,
			6,
			hwnd,
			messageID,
			uintptr(unsafe.Pointer(&iidAsyncOperationUserConsentVerificationResult)),
			uintptr(unsafe.Pointer(&async)),
		)
		if int32(ret) != 0 || async == nil {
			// No async object means the OS request did not start, so this is
			// still a pre-request unavailable path.
			return NewProviderError(ErrorUnavailable)
		}
		defer comRelease(async)
		return waitForConsent(ctx, async)
	})
}

func waitForConsent(ctx context.Context, async unsafe.Pointer) error {
	result, err := waitForAsyncResult(ctx, async)
	if err != nil {
		kind, ok := ProviderErrorKindOf(err)
		if ok && (kind == ErrorCanceled || kind == ErrorTimeout) {
			return err
		}
		return NewProviderError(ErrorVerificationFailed)
	}
	return consentResultError(result)
}

func waitForAsyncResult(ctx context.Context, async unsafe.Pointer) (uint32, error) {
	var info unsafe.Pointer
	ret, _ := comCall(async, 0, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info)))
	if int32(ret) != 0 || info == nil {
		return 0, NewProviderError(ErrorUnavailable)
	}
	defer comRelease(info)

	for {
		var status uint32
		ret, _ := comCall(info, 7, uintptr(unsafe.Pointer(&status)))
		if int32(ret) != 0 {
			return 0, NewProviderError(ErrorUnavailable)
		}
		switch status {
		case consentPending:
			select {
			case <-ctx.Done():
				_, _ = comCall(info, 9)
				return 0, providerErrorForContext(ctx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		case consentCompleted:
			var result uint32
			ret, _ := comCall(async, 8, uintptr(unsafe.Pointer(&result)))
			if int32(ret) != 0 {
				return 0, NewProviderError(ErrorUnavailable)
			}
			return result, nil
		case consentCanceledStatus, consentErrorStatus:
			return 0, NewProviderError(ErrorUnavailable)
		default:
			return 0, NewProviderError(ErrorUnavailable)
		}
	}
}

func consentResultError(result uint32) error {
	switch result {
	case consentVerified:
		return nil
	case consentDeviceBusy:
		return NewProviderError(ErrorDeviceBusy)
	case consentRetriesExhausted:
		return NewProviderError(ErrorLocked)
	case consentCanceled:
		return NewProviderError(ErrorCanceled)
	default:
		// These are results observed after the verification request started.
		// They must not be turned into a second master-password prompt.
		return NewProviderError(ErrorVerificationFailed)
	}
}

func consentAvailability(result uint32) Availability {
	switch result {
	case consentVerified:
		return AvailabilityAvailable
	case consentDeviceNotPresent:
		return AvailabilityDeviceNotPresent
	case consentNotConfigured:
		return AvailabilityNotConfigured
	case consentDisabledByPolicy:
		return AvailabilityDisabledByPolicy
	case consentDeviceBusy:
		return AvailabilityDeviceBusy
	default:
		return AvailabilityUnavailable
	}
}

func consentAvailabilityError(availability Availability) error {
	switch availability {
	case AvailabilityAvailable:
		return nil
	case AvailabilityNotConfigured:
		return NewProviderError(ErrorNotConfigured)
	case AvailabilityDeviceNotPresent, AvailabilityDisabledByPolicy,
		AvailabilityNoActiveWindow, AvailabilityUnsupported:
		return NewProviderError(ErrorUnsupported)
	case AvailabilityDeviceBusy:
		return NewProviderError(ErrorDeviceBusy)
	default:
		return NewProviderError(ErrorUnavailable)
	}
}

func initializeCOM() (initialized, ok bool) {
	ret, _, _ := procRoInitialize.Call(roInitMultithreaded)
	if ret == 0 || ret == 1 {
		return true, true
	}
	// RPC_E_CHANGED_MODE means the thread was already initialized with another
	// apartment. WinRT can still be used by that thread, so continue without a
	// matching RoUninitialize.
	if ret == 0x80010106 {
		return false, true
	}
	return false, false
}

type consoleWindowCandidate struct {
	handle      uintptr
	valid       bool
	visible     bool
	messageOnly bool
}

func (candidate consoleWindowCandidate) usable() bool {
	return candidate.handle != 0 && candidate.valid && candidate.visible && !candidate.messageOnly
}

// selectActiveApplicationWindow deliberately ignores foregroundWindow. A
// foreground window owned by another process is not a safe owner for the
// consent dialog, and a pseudoconsole may expose a hidden message-only
// console window. In either case, returning zero forces the password path.
func selectActiveApplicationWindow(console consoleWindowCandidate, _ uintptr) uintptr {
	if console.usable() {
		return console.handle
	}
	return 0
}

func inspectConsoleWindow(hwnd uintptr) consoleWindowCandidate {
	if hwnd == 0 {
		return consoleWindowCandidate{}
	}
	valid, _, _ := procIsWindow.Call(hwnd)
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	return consoleWindowCandidate{
		handle:      hwnd,
		valid:       valid != 0 && root == hwnd,
		visible:     visible != 0,
		messageOnly: visible == 0,
	}
}

func activeWindow() uintptr {
	hwnd, _, _ := procGetConsoleWindow.Call()
	return selectActiveApplicationWindow(inspectConsoleWindow(hwnd), 0)
}

func requestWithActiveWindow(ctx context.Context, window func() uintptr, request func(uintptr) error) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	hwnd := window()
	if hwnd == 0 {
		return NewProviderError(ErrorUnsupported)
	}
	return request(hwnd)
}

func comCall(instance unsafe.Pointer, slot int, args ...uintptr) (uintptr, syscall.Errno) {
	vtable := *(*unsafe.Pointer)(instance)
	method := *(*uintptr)(unsafe.Add(vtable, uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 1, len(args)+1)
	all[0] = uintptr(instance)
	all = append(all, args...)
	ret, _, callErr := syscall.SyscallN(method, all...)
	return ret, callErr
}

func comRelease(instance unsafe.Pointer) {
	if instance == nil {
		return
	}
	_, _ = comCall(instance, 2)
}
