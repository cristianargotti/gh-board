package commands

import (
	"bytes"
	"crypto/sha256"
	"debug/macho"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
)

// Code signature constants of the Mach-O format (cs_blobs.h of xnu).
const (
	machoCodeSignatureCmd   = 0x1d       // LC_CODE_SIGNATURE
	machoSuperBlobMagic     = 0xfade0cc0 // CSMAGIC_EMBEDDED_SIGNATURE
	machoCodeDirectoryMagic = 0xfade0c02 // CSMAGIC_CODEDIRECTORY
	machoCodeDirectorySlot  = 0          // CSSLOT_CODEDIRECTORY
	machoFlagAdHoc          = 0x2        // CS_ADHOC
	machoFlagLinkerSigned   = 0x20000    // CS_LINKER_SIGNED
	machoHeaderSize64       = 32
	machoMaxSignatureBytes  = 16 << 20
)

// readMachOSignature describes the embedded code signature of a Mach-O
// executable and the header fields that only size it, which re-signing
// rewrites along with the signature.
type readMachOSignature struct {
	Present      bool
	AdHoc        bool
	LinkerSigned bool
	Identifier   string
	headerEnd    int64
	dataOffset   int64
	dataSize     int64
	masks        [][2]int64
}

// Resigned reports an ad hoc signature the Go linker did not write, which
// is what codesign leaves when gh re-signs the binary on Apple silicon.
func (s readMachOSignature) Resigned() bool {
	return s.Present && s.AdHoc && !s.LinkerSigned
}

// String describes the signature for the doctor report.
func (s readMachOSignature) String() string {
	if !s.Present {
		return ""
	}
	var parts []string
	if s.AdHoc {
		parts = append(parts, "ad hoc")
	}
	if s.LinkerSigned {
		parts = append(parts, "linker-signed")
	}
	if s.Identifier != "" {
		parts = append(parts, "identifier "+s.Identifier)
	}
	return strings.Join(parts, ", ")
}

// readMachOSignatureOf inspects the code signature of an executable. A file
// of another format, one without a signature, or one whose signature does
// not parse, reports Present false, so doctor falls back to the plain
// checksum comparison.
func readMachOSignatureOf(path string) readMachOSignature {
	f, err := os.Open(path) //nolint:gosec // the running binary or the asset doctor downloaded
	if err != nil {
		return readMachOSignature{}
	}
	defer func() { _ = f.Close() }()
	signature, err := readMachOInspect(f)
	if err != nil {
		return readMachOSignature{}
	}
	return signature
}

// readMachOInspect walks the load commands for the __LINKEDIT segment and
// the code signature, then reads the code directory of the signature.
func readMachOInspect(r io.ReaderAt) (readMachOSignature, error) {
	f, err := macho.NewFile(r)
	if err != nil {
		return readMachOSignature{}, err
	}
	if f.Magic != macho.Magic64 {
		return readMachOSignature{}, errors.New("not a 64-bit Mach-O file")
	}
	signature := readMachOSignature{headerEnd: machoHeaderSize64 + int64(f.Cmdsz)}
	offset := int64(machoHeaderSize64)
	for _, load := range f.Loads {
		raw := load.Raw()
		if segment, ok := load.(*macho.Segment); ok && segment.Name == "__LINKEDIT" {
			signature.masks = append(signature.masks, [2]int64{offset + 32, offset + 40}, [2]int64{offset + 48, offset + 56})
		}
		if len(raw) >= 16 && f.ByteOrder.Uint32(raw[0:4]) == machoCodeSignatureCmd {
			signature.dataOffset = int64(f.ByteOrder.Uint32(raw[8:12]))
			signature.dataSize = int64(f.ByteOrder.Uint32(raw[12:16]))
			signature.masks = append(signature.masks, [2]int64{offset + 12, offset + 16})
		}
		offset += int64(len(raw))
	}
	if signature.dataOffset < signature.headerEnd || signature.dataSize == 0 || signature.dataSize > machoMaxSignatureBytes {
		return readMachOSignature{}, errors.New("no code signature")
	}
	if err := signature.readDirectory(r); err != nil {
		return readMachOSignature{}, err
	}
	signature.Present = true
	return signature, nil
}

// readDirectory finds the code directory in the signature super blob and
// reads its flags and identifier (big-endian, as the format prescribes).
func (s *readMachOSignature) readDirectory(r io.ReaderAt) error {
	blob := make([]byte, s.dataSize)
	if _, err := r.ReadAt(blob, s.dataOffset); err != nil {
		return err
	}
	be := binary.BigEndian
	if len(blob) < 12 || be.Uint32(blob[0:4]) != machoSuperBlobMagic {
		return errors.New("no embedded signature")
	}
	count := int64(be.Uint32(blob[8:12]))
	for i := int64(0); i < count && 12+8*i+8 <= int64(len(blob)); i++ {
		entry := blob[12+8*i:]
		if be.Uint32(entry[0:4]) != machoCodeDirectorySlot {
			continue
		}
		return s.readCodeDirectory(blob, int64(be.Uint32(entry[4:8])))
	}
	return errors.New("no code directory")
}

// readCodeDirectory reads the flags and identifier of the code directory
// at off within the signature blob.
func (s *readMachOSignature) readCodeDirectory(blob []byte, off int64) error {
	be := binary.BigEndian
	if off+24 > int64(len(blob)) || be.Uint32(blob[off:off+4]) != machoCodeDirectoryMagic {
		return errors.New("damaged code directory")
	}
	flags := be.Uint32(blob[off+12 : off+16])
	s.AdHoc = flags&machoFlagAdHoc != 0
	s.LinkerSigned = flags&machoFlagLinkerSigned != 0
	ident := off + int64(be.Uint32(blob[off+20:off+24]))
	if ident >= int64(len(blob)) {
		return errors.New("damaged code directory identifier")
	}
	end := bytes.IndexByte(blob[ident:], 0)
	if end < 0 {
		return errors.New("damaged code directory identifier")
	}
	s.Identifier = string(blob[ident : ident+int64(end)])
	return nil
}

// readMachOEqualExceptSignature reports whether two signed executables
// hold the same bytes up to their code signature, ignoring the header
// fields that only size the signature. It is how a copy that gh re-signed
// after the download is matched with the release asset.
func readMachOEqualExceptSignature(installed, asset string) (bool, error) {
	a := readMachOSignatureOf(installed)
	b := readMachOSignatureOf(asset)
	if !a.Present || !b.Present || a.dataOffset != b.dataOffset || !slices.Equal(a.masks, b.masks) {
		return false, nil
	}
	ha, err := readMaskedSHA256(installed, a)
	if err != nil {
		return false, err
	}
	hb, err := readMaskedSHA256(asset, b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ha, hb), nil
}

// readMaskedSHA256 hashes the file up to the signature with the masked
// header ranges zeroed.
func readMaskedSHA256(path string, signature readMachOSignature) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // the running binary or the asset doctor downloaded
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, signature.headerEnd)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, err
	}
	for _, m := range signature.masks {
		if m[0] < 0 || m[1] > signature.headerEnd {
			return nil, errors.New("signature fields outside the header")
		}
		clear(head[m[0]:m[1]])
	}
	h := sha256.New()
	_, _ = h.Write(head)
	if _, err := io.CopyN(h, f, signature.dataOffset-signature.headerEnd); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
