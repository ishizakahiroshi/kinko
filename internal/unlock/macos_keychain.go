//go:build darwin && cgo

package unlock

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static const char *kinko_keychain_service = "com.ishizakahiroshi.kinko.unlock";

static CFMutableDictionaryRef kinko_query(const char *account) {
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0, &kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks);
	CFStringRef service = CFStringCreateWithCString(kCFAllocatorDefault,
		kinko_keychain_service, kCFStringEncodingUTF8);
	CFStringRef accountString = CFStringCreateWithCString(kCFAllocatorDefault,
		account, kCFStringEncodingUTF8);
	CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(query, kSecAttrService, service);
	CFDictionarySetValue(query, kSecAttrAccount, accountString);
	CFDictionarySetValue(query, kSecUseDataProtectionKeychain, kCFBooleanTrue);
	CFRelease(service);
	CFRelease(accountString);
	return query;
}

static void kinko_explicit_bzero(void *value, size_t valueLength) {
	if (value == NULL || valueLength == 0) return;
	explicit_bzero(value, valueLength);
}

static CFMutableDataRef kinko_data(const unsigned char *value, size_t valueLength) {
	CFMutableDataRef data = CFDataCreateMutable(kCFAllocatorDefault, (CFIndex)valueLength);
	if (data == NULL) return NULL;
	if (valueLength > 0) {
		CFDataAppendBytes(data, value, (CFIndex)valueLength);
	}
	return data;
}

static void kinko_zero_cf_data(CFMutableDataRef data) {
	if (data == NULL) return;
	CFIndex length = CFDataGetLength(data);
	if (length > 0) {
		kinko_explicit_bzero(CFDataGetMutableBytePtr(data), (size_t)length);
	}
}

int kinko_keychain_write(const char *account, const unsigned char *value,
	 size_t valueLength, bool replace) {
	CFMutableDictionaryRef query = kinko_query(account);
	CFMutableDataRef data = kinko_data(value, valueLength);
	CFMutableDictionaryRef attributes = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0, &kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks);
	if (data == NULL || attributes == NULL) {
		if (attributes != NULL) CFRelease(attributes);
		if (data != NULL) {
			kinko_zero_cf_data(data);
			CFRelease(data);
		}
		CFRelease(query);
		return errSecAllocate;
	}
	CFDictionarySetValue(attributes, kSecValueData, data);

	OSStatus status = errSecSuccess;
	if (replace) {
		status = SecItemUpdate(query, attributes);
		if (status == errSecItemNotFound) {
			replace = false;
		}
	}
	if (!replace) {
		CFErrorRef accessError = NULL;
		SecAccessControlRef access = SecAccessControlCreateWithFlags(
			kCFAllocatorDefault, kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
			kSecAccessControlUserPresence, &accessError);
		if (access == NULL) {
			if (accessError != NULL) CFRelease(accessError);
			status = errSecAuthFailed;
		} else {
			CFDictionarySetValue(attributes, kSecClass, kSecClassGenericPassword);
			CFDictionarySetValue(attributes, kSecAttrService,
				CFDictionaryGetValue(query, kSecAttrService));
			CFDictionarySetValue(attributes, kSecAttrAccount,
				CFDictionaryGetValue(query, kSecAttrAccount));
			CFDictionarySetValue(attributes, kSecUseDataProtectionKeychain,
				kCFBooleanTrue);
			CFDictionarySetValue(attributes, kSecAttrAccessControl, access);
			status = SecItemAdd(attributes, NULL);
			CFRelease(access);
		}
	}
	CFRelease(attributes);
	kinko_zero_cf_data(data);
	CFRelease(data);
	CFRelease(query);
	return (int)status;
}

int kinko_keychain_exists(const char *account) {
	CFMutableDictionaryRef query = kinko_query(account);
	CFDictionarySetValue(query, kSecReturnAttributes, kCFBooleanTrue);
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	if (result != NULL) CFRelease(result);
	CFRelease(query);
	return (int)status;
}

int kinko_keychain_read(const char *account, unsigned char **value,
	 size_t *valueLength) {
	*value = NULL;
	*valueLength = 0;
	CFMutableDictionaryRef query = kinko_query(account);
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	if (status == errSecSuccess && result != NULL &&
		CFGetTypeID(result) == CFDataGetTypeID()) {
		CFDataRef data = (CFDataRef)result;
		CFIndex length = CFDataGetLength(data);
		if (length > 0) {
			unsigned char *copy = (unsigned char *)malloc((size_t)length);
			if (copy == NULL) {
				status = errSecAllocate;
			} else {
				memcpy(copy, CFDataGetBytePtr(data), (size_t)length);
				*value = copy;
				*valueLength = (size_t)length;
			}
		}
	}
	if (result != NULL) CFRelease(result);
	CFRelease(query);
	return (int)status;
}

void kinko_keychain_free(unsigned char *value, size_t valueLength) {
	if (value == NULL) return;
	kinko_explicit_bzero(value, valueLength);
	free(value);
}

int kinko_keychain_delete(const char *account) {
	CFMutableDictionaryRef query = kinko_query(account);
	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	return (int)status;
}
*/
import "C"

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"unsafe"
)

const (
	macOSItemNotFound          = -25300
	macOSDuplicateItem         = -25299
	macOSUserCanceled          = -128
	macOSAuthFailed            = -25293
	macOSInteractionNotAllowed = -25308
	macOSAllocateFailure       = -108
)

// MacOSProvider はKeychain itemへuser-presence access controlを付ける。
// kSecAccessControlUserPresenceはTouch IDが無い場合にdevice passcodeを
// 許容するため、「生体認証のみ」とは表示しない。
type MacOSProvider struct{}

func NewMacOSProvider() Provider { return &MacOSProvider{} }

func (p *MacOSProvider) Name() string { return "macos-keychain-user-presence" }

func (p *MacOSProvider) Level() SecurityLevel { return LevelUserVerified }

func (p *MacOSProvider) Setup(ctx context.Context, route Route, vaultID, password string, replace bool) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	if password == "" || !validMacOSVaultID(vaultID) {
		return NewProviderError(ErrorCorruptEntry)
	}
	account := C.CString(route.Key)
	defer C.free(unsafe.Pointer(account))
	if !replace {
		status := int(C.kinko_keychain_exists(account))
		if status == 0 {
			return NewProviderError(ErrorAlreadyConfigured)
		}
		if status != macOSItemNotFound {
			return macOSProviderError(status)
		}
	}
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	payload, err := json.Marshal(macOSCredentialPayload{VaultID: vaultID, Password: password})
	if err != nil {
		return NewProviderError(ErrorCorruptEntry)
	}
	defer zeroBytes(payload)
	value := C.CBytes(payload)
	defer C.kinko_keychain_free((*C.uchar)(value), C.size_t(len(payload)))
	status := int(C.kinko_keychain_write(account, (*C.uchar)(value), C.size_t(len(payload)), C.bool(replace)))
	return macOSProviderError(status)
}

func (p *MacOSProvider) Unlock(ctx context.Context, route Route) (Credential, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, providerErrorForContext(err)
	}
	account := C.CString(route.Key)
	defer C.free(unsafe.Pointer(account))
	var raw *C.uchar
	var rawLength C.size_t
	status := int(C.kinko_keychain_read(account, &raw, &rawLength))
	if status != 0 {
		return Credential{}, macOSProviderError(status)
	}
	defer C.kinko_keychain_free(raw, rawLength)
	if err := ctx.Err(); err != nil {
		return Credential{}, providerErrorForContext(err)
	}
	payload := C.GoBytes(unsafe.Pointer(raw), C.int(rawLength))
	defer zeroBytes(payload)
	var stored macOSCredentialPayload
	if err := json.Unmarshal(payload, &stored); err != nil {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	if stored.Password == "" || !validMacOSVaultID(stored.VaultID) {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	return Credential{Password: stored.Password, VaultID: stored.VaultID}, nil
}

func (p *MacOSProvider) Status(ctx context.Context, route Route) Status {
	status := Status{
		Provider:     p.Name(),
		Level:        p.Level(),
		Binding:      BindingCredentialBound,
		Availability: AvailabilityAvailable,
		Available:    ctx.Err() == nil,
	}
	if !status.Available {
		status.Availability = availabilityForProviderError(providerErrorForContext(ctx.Err()))
		return status
	}
	account := C.CString(route.Key)
	defer C.free(unsafe.Pointer(account))
	code := int(C.kinko_keychain_exists(account))
	switch code {
	case 0:
		status.Configured = true
	case macOSItemNotFound:
		status.Configured = false
	default:
		status.Available = false
		status.Availability = AvailabilityUnavailable
	}
	return status
}

func (p *MacOSProvider) Disable(ctx context.Context, route Route) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	account := C.CString(route.Key)
	defer C.free(unsafe.Pointer(account))
	return macOSProviderError(int(C.kinko_keychain_delete(account)))
}

type macOSCredentialPayload struct {
	VaultID  string `json:"vault_id"`
	Password string `json:"password"`
}

func validMacOSVaultID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func macOSProviderError(status int) error {
	switch status {
	case 0:
		return nil
	case macOSItemNotFound:
		return NewProviderError(ErrorNotConfigured)
	case macOSDuplicateItem:
		return NewProviderError(ErrorAlreadyConfigured)
	case macOSUserCanceled:
		return NewProviderError(ErrorCanceled)
	case macOSAuthFailed:
		return NewProviderError(ErrorLocked)
	case macOSInteractionNotAllowed:
		return NewProviderError(ErrorUnavailable)
	case macOSAllocateFailure:
		return NewProviderError(ErrorUnavailable)
	default:
		return NewProviderError(ErrorUnavailable)
	}
}
