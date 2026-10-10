package inspect

import (
	"encoding/binary"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// kinds lists a map's regions as "kind[start,end)".
func kinds(m SectorMap) string {
	var b strings.Builder
	for _, r := range m.Regions {
		b.WriteString(r.Kind)
		b.WriteString("[")
		b.WriteString(strconv.Itoa(r.Start))
		b.WriteString(",")
		b.WriteString(strconv.Itoa(r.End))
		b.WriteString(") ")
	}
	return strings.TrimSpace(b.String())
}

// contiguous checks the invariant every map keeps: regions in order, no gaps, no overlap,
// covering every mapped byte.
func contiguous(t *testing.T, m SectorMap) {
	t.Helper()
	at := 0
	for _, r := range m.Regions {
		if r.Start != at || r.End <= r.Start {
			t.Fatalf("regions not contiguous at %d: %s", at, kinds(m))
		}
		at = r.End
	}
	if want := min(m.Size, 512); at != want {
		t.Fatalf("regions cover %d of %d bytes: %s", at, want, kinds(m))
	}
}

func TestMapPlainBootSector(t *testing.T) {
	s := make([]byte, 512)
	copy(s, []byte{0xFA, 0x31, 0xC0, 0x8E, 0xD8, 0xF4, 0xEB, 0xFD})
	s[510], s[511] = 0x55, 0xAA
	m := MapSector(s)
	contiguous(t, m)
	if got := kinds(m); got != "code[0,8) padding[8,510) signature[510,512)" {
		t.Errorf("regions %s", got)
	}
	if !m.SignatureOK || m.Signature != [2]byte{0x55, 0xAA} || !m.Regions[2].Known || m.Regions[0].Known {
		t.Errorf("signature %x ok %v; known flags %v %v", m.Signature, m.SignatureOK, m.Regions[0].Known, m.Regions[2].Known)
	}
	if !strings.Contains(m.Regions[1].Label, "502 bytes free") {
		t.Errorf("padding label %q", m.Regions[1].Label)
	}

	s[511] = 0x00
	if m := MapSector(s); m.SignatureOK || !strings.Contains(m.Regions[2].Detail, "invalid") {
		t.Errorf("bad signature: %+v", m.Regions[2])
	}

	// A disk image longer than a sector maps its first sector.
	img := append(append([]byte(nil), s...), make([]byte, 1024)...)
	if m := MapSector(img); m.Size != 1536 {
		t.Errorf("image size %d", m.Size)
	} else {
		contiguous(t, m)
	}
}

func TestMapShortAndEmpty(t *testing.T) {
	m := MapSector([]byte{0xF4})
	contiguous(t, m)
	if len(m.Regions) != 1 || m.Regions[0].Kind != "short" || m.SignatureOK {
		t.Errorf("short image: %+v", m)
	}
	if m := MapSector(nil); len(m.Regions) != 0 || m.SignatureOK {
		t.Errorf("empty image: %+v", m)
	}
}

func TestMapFATBootSector(t *testing.T) {
	s := make([]byte, 512)
	copy(s, []byte{0xEB, 0x3C, 0x90})
	copy(s[3:], "MSDOS5.0")
	binary.LittleEndian.PutUint16(s[11:], 512) // bytes per sector
	s[13] = 1                                  // sectors per cluster
	binary.LittleEndian.PutUint16(s[14:], 1)   // reserved sectors
	s[16] = 2                                  // FATs
	binary.LittleEndian.PutUint16(s[22:], 9)   // sectors per FAT: FAT12
	s[38] = 0x29                               // extended boot signature
	copy(s[62:], []byte{0xFA, 0x31, 0xC0, 0xF4})
	s[510], s[511] = 0x55, 0xAA
	m := MapSector(s)
	contiguous(t, m)
	if got := kinds(m); got != "jump[0,3) oem[3,11) bpb[11,36) ebpb[36,62) code[62,66) padding[66,510) signature[510,512)" {
		t.Errorf("regions %s", got)
	}
	if m.Regions[2].Known || !strings.HasPrefix(m.Regions[2].Detail, "detected") || m.Regions[1].Label != `OEM name "MSDOS5.0"` {
		t.Errorf("BPB region %+v / %+v", m.Regions[1], m.Regions[2])
	}

	// FAT32: sectors per FAT is 0 and the extended block is longer.
	binary.LittleEndian.PutUint16(s[22:], 0)
	s[38], s[66] = 0, 0x29
	if got := kinds(MapSector(s)); !strings.Contains(got, "ebpb[36,90)") {
		t.Errorf("FAT32 regions %s", got)
	}
}

func TestMapMBR(t *testing.T) {
	s := make([]byte, 512)
	copy(s, []byte{0xFA, 0x31, 0xC0, 0xF4})
	binary.LittleEndian.PutUint32(s[440:], 0xDEADBEEF)
	e := s[446:]
	e[0], e[4] = 0x80, 0x0C
	binary.LittleEndian.PutUint32(e[8:], 2048)
	binary.LittleEndian.PutUint32(e[12:], 100000)
	s[510], s[511] = 0x55, 0xAA
	m := MapSector(s)
	contiguous(t, m)
	if got := kinds(m); got != "code[0,4) padding[4,440) disk-signature[440,446) partition-table[446,510) signature[510,512)" {
		t.Errorf("regions %s", got)
	}
	if len(m.Partitions) != 4 || !m.Partitions[0].Bootable || m.Partitions[0].LBA != 2048 || m.Partitions[0].Type != 0x0C ||
		!strings.Contains(m.Regions[3].Label, "1 in use") {
		t.Errorf("partitions %+v, label %q", m.Partitions, m.Regions[3].Label)
	}

	// An invalid status byte means the bytes are not a partition table.
	e[16] = 0x13
	if got := kinds(MapSector(s)); strings.Contains(got, "partition") {
		t.Errorf("garbage taken for a table: %s", got)
	}
}

// TestMapRandomBytes feeds arbitrary sectors: the map must stay well formed whatever it finds.
func TestMapRandomBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		s := make([]byte, 480+r.IntN(64))
		for i := range s {
			if r.IntN(3) > 0 {
				s[i] = byte(r.IntN(256))
			}
		}
		contiguous(t, MapSector(s))
	}
}
