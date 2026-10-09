package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Code directory flags of the two signatures a kit binary carries: the
// one the Go linker writes and the one codesign leaves when gh re-signs
// the download on Apple silicon.
const (
	readLinkerFlags   = uint32(machoFlagAdHoc | machoFlagLinkerSigned)
	readResignedFlags = uint32(machoFlagAdHoc)
)

var readBinaryPayload = []byte("the code of the kit\n")

// readFakeMachO builds a minimal 64-bit Mach-O executable: a __LINKEDIT
// segment command, an LC_CODE_SIGNATURE command, the payload and an
// embedded signature whose code directory carries flags and identifier.
func readFakeMachO(payload []byte, flags uint32, identifier string) []byte {
	return readFakeMachOWithBlob(payload, readFakeSignatureBlob(flags, identifier))
}

func readFakeMachOWithBlob(payload, blob []byte) []byte {
	const headerEnd = machoHeaderSize64 + 72 + 16
	le := binary.LittleEndian
	var b bytes.Buffer
	// magic, cputype arm64, subtype, MH_EXECUTE, ncmds, sizeofcmds, flags, reserved
	for _, v := range []uint32{0xfeedfacf, 0x0100000c, 0, 2, 2, 72 + 16, 0, 0} {
		_ = binary.Write(&b, le, v)
	}
	_ = binary.Write(&b, le, []uint32{0x19, 72}) // LC_SEGMENT_64
	name := make([]byte, 16)
	copy(name, "__LINKEDIT")
	b.Write(name)
	size := uint64(len(payload) + len(blob))
	_ = binary.Write(&b, le, []uint64{0x100000000, size, headerEnd, size}) // vmaddr, vmsize, fileoff, filesize
	_ = binary.Write(&b, le, []uint32{1, 1, 0, 0})                         // maxprot, initprot, nsects, flags
	_ = binary.Write(&b, le, []uint32{machoCodeSignatureCmd, 16, readU32(headerEnd + len(payload)), readU32(len(blob))})
	b.Write(payload)
	b.Write(blob)
	return b.Bytes()
}

// readFakeSignatureBlob builds a super blob holding one code directory.
func readFakeSignatureBlob(flags uint32, identifier string) []byte {
	ident := []byte(identifier + "\x00")
	be := binary.BigEndian
	var cd bytes.Buffer
	length := readU32(44 + len(ident))
	// magic, length, version, flags, hashOffset, identOffset, nSpecialSlots, nCodeSlots, codeLimit
	_ = binary.Write(&cd, be, []uint32{machoCodeDirectoryMagic, length, 0x20400, flags, length, 44, 0, 0, 0})
	cd.Write([]byte{32, 2, 0, 12}) // hashSize, hashType, platform, pageSize
	_ = binary.Write(&cd, be, uint32(0))
	cd.Write(ident)
	var blob bytes.Buffer
	_ = binary.Write(&blob, be, []uint32{machoSuperBlobMagic, readU32(20 + cd.Len()), 1, machoCodeDirectorySlot, 20})
	blob.Write(cd.Bytes())
	return blob.Bytes()
}

// readU32 narrows a fixture size, which is a few hundred bytes.
func readU32(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		panic("fixture size out of range")
	}
	return uint32(n) //nolint:gosec // checked above
}

func readSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readWriteBinary(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type readBinaryCase struct {
	name      string
	installed []byte
	release   string
	download  []byte
	err       error
	want      []string
	unwanted  []string
}

func readBinaryCases() []readBinaryCase {
	asset := readFakeMachO(readBinaryPayload, readLinkerFlags, "a.out")
	resigned := readFakeMachO(readBinaryPayload, readResignedFlags, "gh-board-1234")
	tampered := readFakeMachO([]byte("the code of a tampered kit\n"), readResignedFlags, "gh-board-1234")
	pointer := "gh attestation verify gh-board_1.2.3_"
	return []readBinaryCase{
		{"resigned copy verified", resigned, readSHA256Hex(asset), asset, nil, []string{
			"Signature:         ad hoc, identifier gh-board-1234", "Asset SHA-256:     " + readSHA256Hex(asset),
			"Provenance:        release asset matches the manifest", "Verified:          yes",
			"installed copy re-signed by macOS after download", "matches the release asset apart from the code signature",
			pointer, "--repo acme/gh-board", "compare its SHA-256 with checksums.txt", "Problems\n  none",
		}, []string{"checksum differs"}},
		{"resigned copy tampered", tampered, readSHA256Hex(asset), asset, nil, []string{
			"Verified:          no", "binary: installed copy differs from the release asset beyond the code signature",
		}, []string{"checksum differs"}},
		{"resigned copy offline", resigned, readSHA256Hex(asset), nil, errors.New("offline"), []string{
			"Provenance:        release asset not downloaded: offline", "not compared with the release asset", pointer, "Problems\n  none",
		}, []string{"Verified:", "checksum differs"}},
		{"release asset damaged", resigned, readSHA256Hex(asset), tampered, nil, []string{
			"Verified:          no", "binary: release asset differs from the manifest", "not compared with the release asset",
		}, []string{"checksum differs"}},
		{"linker signed copy differs", asset, readSHA256Hex(tampered), tampered, nil, []string{
			"Signature:         ad hoc, linker-signed, identifier a.out", "release asset matches the manifest",
			"Verified:          no", "binary: checksum differs from the release checksum",
		}, []string{"re-signed"}},
		{"plain file differs", []byte("not a Mach-O file\n"), "deadbeef", []byte("x"), nil, []string{
			"Verified:          no", "binary: checksum differs from the release checksum", "binary: release asset differs from the manifest",
		}, []string{"Signature:", "re-signed"}},
	}
}

func TestReadDoctorBinary(t *testing.T) {
	for _, tc := range readBinaryCases() {
		t.Run(tc.name, func(t *testing.T) {
			readPinEnvironment(t)
			temp := readIsolateTemp(t)
			readStubExecutable(t, readWriteBinary(t, "gh-board", tc.installed))
			fake := &readFakeDownloads{readFakeChecksums: &readFakeChecksums{readFakeReader: readNewFakeReader(), sum: tc.release}, data: tc.download, err: tc.err}
			deps, out := readTestDeps(t, fake)
			if err := readRun(t, deps, "doctor"); err != nil {
				t.Fatal(err)
			}
			readAssertOutput(t, out.String(), tc.want, tc.unwanted)
			if left, _ := filepath.Glob(filepath.Join(temp, "gh-board-asset-*")); len(left) != 0 {
				t.Errorf("temporary asset left behind: %v", left)
			}
		})
	}
}

// readIsolateTemp points the temporary directory of every platform at a
// fresh directory, so the test sees what doctor leaves behind.
func readIsolateTemp(t *testing.T) string {
	t.Helper()
	temp := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, temp)
	}
	return temp
}

func readAssertOutput(t *testing.T, out string, want, unwanted []string) {
	t.Helper()
	for _, text := range want {
		if !strings.Contains(out, text) {
			t.Errorf("output lacks %q:\n%s", text, out)
		}
	}
	for _, text := range unwanted {
		if strings.Contains(out, text) {
			t.Errorf("output has %q:\n%s", text, out)
		}
	}
}

func TestReadDoctorBinaryJSON(t *testing.T) {
	readPinEnvironment(t)
	asset := readFakeMachO(readBinaryPayload, readLinkerFlags, "a.out")
	readStubExecutable(t, readWriteBinary(t, "gh-board", readFakeMachO(readBinaryPayload, readResignedFlags, "gh-board-1234")))
	fake := &readFakeDownloads{readFakeChecksums: &readFakeChecksums{readFakeReader: readNewFakeReader(), sum: readSHA256Hex(asset)}, data: asset}
	deps, out := readTestDeps(t, fake)
	if err := readRun(t, deps, "doctor", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"signature":"ad hoc, identifier gh-board-1234"`, `"provenance":"release asset matches the manifest"`, `"verified":true`, `"problems":[]`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("json lacks %s:\n%s", want, out.String())
		}
	}
}

// Damaged signatures report no signature at all, so doctor falls back to
// the plain checksum comparison instead of excusing a difference.
func readDamagedBlobs() map[string][]byte {
	good := readFakeSignatureBlob(readResignedFlags, "gh-board-1234")
	patch := func(offset int, value uint32) []byte {
		blob := bytes.Clone(good)
		binary.BigEndian.PutUint32(blob[offset:], value)
		return blob
	}
	return map[string][]byte{
		"super blob magic":   patch(0, 0),
		"no code directory":  patch(8, 0),
		"directory magic":    patch(20, 0),
		"identifier offset":  patch(40, 0xffff),
		"identifier not nul": bytes.TrimSuffix(good, []byte{0}),
		"truncated":          good[:30],
		"empty":              {},
	}
}

func TestReadMachOSignature(t *testing.T) {
	linker := readMachOSignatureOf(readWriteBinary(t, "asset", readFakeMachO(readBinaryPayload, readLinkerFlags, "a.out")))
	if !linker.Present || !linker.AdHoc || !linker.LinkerSigned || linker.Resigned() || linker.String() != "ad hoc, linker-signed, identifier a.out" {
		t.Fatalf("linker signature = %+v", linker)
	}
	resigned := readMachOSignatureOf(readWriteBinary(t, "installed", readFakeMachO(readBinaryPayload, readResignedFlags, "gh-board-1234")))
	if !resigned.Resigned() || resigned.String() != "ad hoc, identifier gh-board-1234" {
		t.Fatalf("re-signed signature = %+v", resigned)
	}
	plain := readFakeMachO(readBinaryPayload, 0, "")
	if got := readMachOSignatureOf(readWriteBinary(t, "plain", plain)); !got.Present || got.String() != "" || got.Resigned() {
		t.Fatalf("plain signature = %+v", got)
	}
	absent := map[string][]byte{"text": []byte("not a Mach-O file\n"), "short": plain[:40], "cut signature": plain[:len(plain)-5]}
	absent["32-bit magic"] = append([]byte{0xce, 0xfa, 0xed, 0xfe}, plain[4:]...)
	for name, blob := range readDamagedBlobs() {
		absent[name] = readFakeMachOWithBlob(readBinaryPayload, blob)
	}
	for name, data := range absent {
		if got := readMachOSignatureOf(readWriteBinary(t, "file", data)); got.Present {
			t.Errorf("%s: signature reported: %+v", name, got)
		}
	}
	if got := readMachOSignatureOf(filepath.Join(t.TempDir(), "missing")); got.Present {
		t.Errorf("missing file: signature reported: %+v", got)
	}
}

func TestReadMachOEqualExceptSignature(t *testing.T) {
	asset := readWriteBinary(t, "asset", readFakeMachO(readBinaryPayload, readLinkerFlags, "a.out"))
	resigned := readWriteBinary(t, "installed", readFakeMachO(readBinaryPayload, readResignedFlags, "gh-board-1234"))
	tampered := readWriteBinary(t, "tampered", readFakeMachO([]byte("the code of a tampered kit\n"), readResignedFlags, "gh-board-1234"))
	text := readWriteBinary(t, "text", []byte("not a Mach-O file\n"))
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"re-signed copy", resigned, asset, true},
		{"same file", asset, asset, true},
		{"tampered copy", tampered, asset, false},
		{"not signed", text, asset, false},
		{"missing", filepath.Join(t.TempDir(), "x"), asset, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readMachOEqualExceptSignature(tc.a, tc.b)
			if err != nil || got != tc.want {
				t.Fatalf("equal = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := readMaskedSHA256(asset, readMachOSignature{headerEnd: 10, masks: [][2]int64{{0, 20}}}); err == nil {
		t.Fatal("a mask outside the header must fail")
	}
	if _, err := readMaskedSHA256(text, readMachOSignature{headerEnd: 1 << 20}); err == nil {
		t.Fatal("a header past the end of the file must fail")
	}
}
