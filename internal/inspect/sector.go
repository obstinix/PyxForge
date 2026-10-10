package inspect

import (
	"encoding/binary"
	"fmt"
)

// Region is a byte range of a boot sector, [Start, End).
type Region struct {
	Start, End int
	Kind       string // "code", "jump", "oem", "bpb", "ebpb", "disk-signature", "partition-table", "padding", "signature", "short"
	Label      string
	// Known is true when the bytes alone establish the region (the signature, zero padding).
	// Detected regions (a FAT parameter block, a partition table) match a known layout that the
	// bytes could also match by chance; Detail says what was checked.
	Known  bool
	Detail string
}

// Size is the region's length in bytes.
func (r Region) Size() int { return r.End - r.Start }

// Partition is one entry of an MBR partition table.
type Partition struct {
	Bootable bool
	Type     byte
	LBA      uint32
	Sectors  uint32
}

// SectorMap describes the first sector of an image.
type SectorMap struct {
	Size        int // the image's size; the map covers min(Size, 512) bytes
	Regions     []Region
	Signature   [2]byte // the bytes at 0x1FE and 0x1FF
	SignatureOK bool
	Partitions  []Partition
}

// MapSector lays out the first 512 bytes of an image. The regions are contiguous and cover
// every byte mapped. Code and data are left as one region: without symbols they cannot be told
// apart reliably, and the Disasm view decodes the bytes as code on request.
func MapSector(data []byte) SectorMap {
	m := SectorMap{Size: len(data)}
	if len(data) < 512 {
		if len(data) > 0 {
			m.Regions = []Region{{Start: 0, End: len(data), Kind: "short", Known: true,
				Label: "Image shorter than a sector", Detail: fmt.Sprintf("%d of 512 bytes; a BIOS loads whole sectors and needs 55 aa at 0x1FE", len(data))}}
		}
		return m
	}
	s := data[:512]
	m.Signature = [2]byte{s[510], s[511]}
	m.SignatureOK = s[510] == 0x55 && s[511] == 0xAA

	var head []Region // structure at the start
	codeStart := 0
	if bpbEnd, detail, ok := fatBPB(s); ok {
		head = append(head,
			Region{Start: 0, End: 3, Kind: "jump", Label: "Jump over the parameter block", Detail: detail},
			Region{Start: 3, End: 11, Kind: "oem", Label: fmt.Sprintf("OEM name %q", printable(s[3:11])), Detail: detail},
			Region{Start: 11, End: 36, Kind: "bpb", Label: "BIOS parameter block (FAT)", Detail: detail})
		if bpbEnd > 36 {
			head = append(head, Region{Start: 36, End: bpbEnd, Kind: "ebpb", Label: "Extended parameter block", Detail: detail})
		}
		codeStart = bpbEnd
	}

	tail := []Region{{Start: 510, End: 512, Kind: "signature", Known: true, Label: "Boot signature",
		Detail: fmt.Sprintf("%02x %02x: %s", s[510], s[511], map[bool]string{true: "valid, a BIOS boots this sector", false: "invalid, a BIOS skips this sector (55 aa expected)"}[m.SignatureOK])}}
	codeEnd := 510
	if parts, ok := partitionTable(s); ok && codeStart == 0 {
		m.Partitions = parts
		detail := "four 16-byte entries with valid status bytes (00 or 80) and at least one partition"
		tail = append([]Region{
			{Start: 440, End: 446, Kind: "disk-signature", Label: "Disk signature", Detail: "MBR layout, detected with the partition table"},
			{Start: 446, End: 510, Kind: "partition-table", Label: fmt.Sprintf("Partition table (%d in use)", inUse(parts)), Detail: detail},
		}, tail...)
		codeEnd = 440
	}

	// Zero bytes just before the structure at the end are padding.
	pad := codeEnd
	for pad > codeStart && s[pad-1] == 0 {
		pad--
	}
	regions := head
	if pad > codeStart {
		regions = append(regions, Region{Start: codeStart, End: pad, Kind: "code", Label: "Code and data",
			Detail: "machine code and data are not told apart; the Disasm view decodes these bytes as code"})
	}
	if codeEnd > pad {
		regions = append(regions, Region{Start: pad, End: codeEnd, Kind: "padding", Known: true,
			Label: fmt.Sprintf("Zero padding (%d bytes free)", codeEnd-pad)})
	}
	m.Regions = append(regions, tail...)
	return m
}

// fatBPB recognises a FAT boot sector: a jump at offset 0 and plausible geometry. It returns
// where the parameter block ends.
func fatBPB(s []byte) (end int, detail string, ok bool) {
	jump := (s[0] == 0xEB && s[2] == 0x90) || s[0] == 0xE9
	bytesPerSector := binary.LittleEndian.Uint16(s[11:])
	perCluster := s[13]
	reserved := binary.LittleEndian.Uint16(s[14:])
	fats := s[16]
	if !jump || (bytesPerSector != 512 && bytesPerSector != 1024 && bytesPerSector != 2048 && bytesPerSector != 4096) ||
		perCluster == 0 || perCluster&(perCluster-1) != 0 || reserved == 0 || (fats != 1 && fats != 2) {
		return 0, "", false
	}
	detail = fmt.Sprintf("detected: a jump at 0, %d bytes per sector, %d per cluster, %d reserved, %d FATs",
		bytesPerSector, perCluster, reserved, fats)
	end = 36
	sectorsPerFAT16 := binary.LittleEndian.Uint16(s[22:])
	switch {
	case sectorsPerFAT16 == 0 && (s[66] == 0x28 || s[66] == 0x29): // FAT32 extended block
		end = 90
	case s[38] == 0x28 || s[38] == 0x29: // FAT12/16 extended block
		end = 62
	}
	return end, detail, true
}

// partitionTable reads the four MBR entries at 0x1BE when they look like a partition table.
func partitionTable(s []byte) ([]Partition, bool) {
	var parts []Partition
	used := 0
	for i := range 4 {
		e := s[446+16*i : 446+16*(i+1)]
		if e[0] != 0x00 && e[0] != 0x80 {
			return nil, false
		}
		p := Partition{Bootable: e[0] == 0x80, Type: e[4], LBA: binary.LittleEndian.Uint32(e[8:]), Sectors: binary.LittleEndian.Uint32(e[12:])}
		if p.Type != 0 {
			if p.Sectors == 0 {
				return nil, false
			}
			used++
		} else if p.Bootable || p.LBA != 0 || p.Sectors != 0 {
			return nil, false
		}
		parts = append(parts, p)
	}
	return parts, used > 0
}

func inUse(parts []Partition) int {
	n := 0
	for _, p := range parts {
		if p.Type != 0 {
			n++
		}
	}
	return n
}

func printable(b []byte) string {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 0x20 && c <= 0x7e {
			out[i] = c
		} else {
			out[i] = '.'
		}
	}
	return string(out)
}
