package unlock

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMacOSKeychainTemporaryCFDataIsExplicitlyZeroed(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "macos_keychain.go"))
	if err != nil {
		t.Fatalf("read macOS adapter source: %v", err)
	}
	// Git may check out Go source with CRLF on Windows. Normalize only line
	// endings; the zero-before-release sequence remains mandatory.
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	for _, required := range [][]byte{
		[]byte("CFDataCreateMutable"),
		[]byte("CFDataGetMutableBytePtr"),
		[]byte("#define __STDC_WANT_LIB_EXT1__ 1"),
		[]byte("memset_s(value, valueLength, 0, valueLength)"),
		[]byte("kinko_zero_cf_data(data)"),
	} {
		if !bytes.Contains(source, required) {
			t.Fatalf("macOS adapter is missing zeroable CFData contract %q", required)
		}
	}
	if bytes.Contains(source, []byte("CFDataCreate(kCFAllocatorDefault, value")) {
		t.Fatal("macOS adapter must not keep an immutable CFData password copy")
	}
	if !bytes.Contains(source, []byte("CFRelease(attributes);\n\tkinko_zero_cf_data(data);\n\tCFRelease(data);")) {
		t.Fatal("macOS adapter must zero the temporary CFData before releasing it")
	}
}
