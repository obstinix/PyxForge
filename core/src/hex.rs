use serde::Serialize;
use std::fs;
use std::path::Path;

#[derive(Debug, Serialize, Clone)]
pub struct HexDumpData {
    pub is_boot_sector: bool,
    pub has_boot_signature: bool,
    pub file_size: usize,
    pub lines: Vec<HexDumpLine>,
}

#[derive(Debug, Serialize, Clone)]
pub struct HexDumpLine {
    pub offset: usize,
    pub hex_bytes: Vec<String>,
    pub ascii: String,
}

pub fn format_hex_dump(file_path: &Path) -> Result<HexDumpData, String> {
    let bytes = fs::read(file_path)
        .map_err(|e| format!("Failed to read file '{}': {}", file_path.display(), e))?;

    let file_size = bytes.len();
    let is_boot_sector = file_size == 512;

    // Check if the final two bytes are 0x55 and 0xAA (little endian representation of 0xAA55)
    let has_boot_signature = is_boot_sector && bytes[510] == 0x55 && bytes[511] == 0xaa;

    let mut lines = Vec::new();
    for (i, chunk) in bytes.chunks(16).enumerate() {
        let offset = i * 16;
        let hex_bytes: Vec<String> = chunk.iter().map(|b| format!("{:02x}", b)).collect();

        let ascii: String = chunk
            .iter()
            .map(|&b| {
                if (0x20..=0x7e).contains(&b) {
                    b as char
                } else {
                    '.'
                }
            })
            .collect();

        lines.push(HexDumpLine {
            offset,
            hex_bytes,
            ascii,
        });
    }

    Ok(HexDumpData {
        is_boot_sector,
        has_boot_signature,
        file_size,
        lines,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_hex_dump_empty() {
        let test_file = Path::new("target/test_empty.bin");
        fs::write(test_file, []).unwrap();

        let dump = format_hex_dump(test_file).unwrap();
        assert_eq!(dump.file_size, 0);
        assert!(!dump.is_boot_sector);
        assert!(!dump.has_boot_signature);
        assert!(dump.lines.is_empty());

        let _ = fs::remove_file(test_file);
    }

    #[test]
    fn test_format_hex_dump_valid_bootloader() {
        let test_file = Path::new("target/test_bootloader.bin");
        let mut data = vec![0u8; 512];
        data[0] = 0xeb; // jmp instruction mockup
        data[1] = 0x3c;
        data[510] = 0x55; // Valid boot signature
        data[511] = 0xaa;
        fs::write(test_file, data).unwrap();

        let dump = format_hex_dump(test_file).unwrap();
        assert_eq!(dump.file_size, 512);
        assert!(dump.is_boot_sector);
        assert!(dump.has_boot_signature);
        assert_eq!(dump.lines.len(), 32); // 512 / 16 = 32 lines

        assert_eq!(dump.lines[0].offset, 0);
        assert_eq!(dump.lines[0].hex_bytes[0], "eb");
        assert_eq!(dump.lines[0].hex_bytes[1], "3c");

        assert_eq!(dump.lines[31].offset, 496);
        assert_eq!(dump.lines[31].hex_bytes[14], "55");
        assert_eq!(dump.lines[31].hex_bytes[15], "aa");

        let _ = fs::remove_file(test_file);
    }

    #[test]
    fn test_format_hex_dump_invalid_bootloader_signature() {
        let test_file = Path::new("target/test_invalid_bootloader.bin");
        let mut data = vec![0u8; 512];
        data[510] = 0x00; // Invalid boot signature
        data[511] = 0x00;
        fs::write(test_file, data).unwrap();

        let dump = format_hex_dump(test_file).unwrap();
        assert_eq!(dump.file_size, 512);
        assert!(dump.is_boot_sector);
        assert!(!dump.has_boot_signature);

        let _ = fs::remove_file(test_file);
    }
}
