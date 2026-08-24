use egui::{RichText, Rounding, ScrollArea, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, StitchTheme};

pub fn render_hex_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.vertical(|ui| {
        // -------------------------------------------------------------------
        // 1. Hex Explorer Top Toolbar & Signature Badge
        // -------------------------------------------------------------------
        let top_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), 60.0));
        draw_doodle_panel(ui, top_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), 60.0),
            egui::Layout::left_to_right(egui::Align::Center),
            |ui| {
                ui.add_space(12.0);
                ui.label(RichText::new("🔍 HEX EXPLORER & BINARY INSPECTOR").strong().size(13.0).color(theme.primary));
                ui.add_space(8.0);
                
                if state.is_boot_signature_valid {
                    draw_highlighter_badge(ui, "● BOOT SIGNATURE 0xAA55 (VALID MBR)", theme.success, theme.on_primary);
                } else {
                    draw_highlighter_badge(ui, "○ INVALID SIGNATURE", theme.error, theme.on_primary);
                }

                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    ui.add_space(12.0);
                    if ui.button("📂 Load Binary").clicked() {
                        if let Some(path) = rfd::FileDialog::new().add_filter("Binary", &["bin", "img", "iso", "o"]).pick_file() {
                            state.hex_file_path = path.to_string_lossy().to_string();
                            state.add_log("Hex", &format!("Loaded binary: {}", state.hex_file_path), false);
                        }
                    }
                    ui.label(RichText::new(format!("File: {} (512 Bytes)", state.hex_file_path)).size(11.0).color(theme.text_muted));
                });
            },
        );

        ui.add_space(8.0);

        // -------------------------------------------------------------------
        // 2. Full 16-Column Hex Dump
        // -------------------------------------------------------------------
        let dump_height = ui.available_height().max(250.0);
        let dump_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), dump_height));
        draw_doodle_panel(ui, dump_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), dump_height),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("OFFSET     00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F   DECODED ASCII TEXT").strong().monospace().size(12.0).color(theme.text_muted));
                });
                ui.add_space(4.0);
                ui.separator();

                ScrollArea::vertical()
                    .id_salt("hex_dump_scroll")
                    .show(ui, |ui| {
                        ui.add_space(4.0);
                        
                        let hex_rows = [
                            ("00000000", "FA 31 C0 8E D8 8E C0 8E  D0 BC 00 7C FB E8 0A 00", ".1........|....."),
                            ("00000010", "EB FE AC 08 C0 74 06 B4  0E CD 10 EB F5 C3 BE 20", ".....t......... "),
                            ("00000020", "7C E8 F0 FF F4 3E 3E 20  50 79 78 46 6F 72 67 65", "|....>> PyxForge"),
                            ("00000030", "20 4F 53 20 45 6E 67 69  6E 65 20 76 30 2E 31 2E", " OS Engine v0.1."),
                            ("00000040", "30 20 4C 6F 61 64 65 64  20 3C 3C 0D 0A 00 45 72", "0 Loaded <<...Er"),
                            ("00000050", "72 6F 72 3A 20 49 6E 76  61 6C 69 64 20 42 6F 6F", "ror: Invalid Boo"),
                            ("00000060", "74 20 53 65 63 74 6F 72  20 53 69 67 6E 61 74 75", "t Sector Signatu"),
                            ("00000070", "72 65 21 0D 0A 00 00 00  00 00 00 00 00 00 00 00", "re!............."),
                            ("00000080", "00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00", "................"),
                            ("000001E0", "00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00", "................"),
                            ("000001F0", "00 00 00 00 00 00 00 00  00 00 00 00 00 00 55 AA", "..............U."),
                        ];

                        for (off, hex, ascii) in hex_rows {
                            ui.horizontal(|ui| {
                                ui.add_space(12.0);
                                ui.label(RichText::new(off).monospace().size(12.0).color(theme.text_muted));
                                
                                if off == "000001F0" {
                                    ui.label(RichText::new("00 00 00 00 00 00 00 00  00 00 00 00 00 00 ").monospace().size(12.0).color(theme.primary));
                                    ui.label(RichText::new("55 AA").strong().monospace().size(12.0).color(theme.success));
                                } else {
                                    ui.label(RichText::new(hex).monospace().size(12.0).color(theme.primary));
                                }

                                ui.label(RichText::new(format!(" | {} |", ascii)).monospace().size(12.0).color(theme.secondary));
                            });
                            ui.add_space(2.0);
                        }
                    });
            },
        );
    });
}
