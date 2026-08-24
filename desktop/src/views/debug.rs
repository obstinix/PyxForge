use egui::{Color32, RichText, Rounding, ScrollArea, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, StitchTheme};

pub fn render_debug_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.vertical(|ui| {
        // -------------------------------------------------------------------
        // 1. Top Debug Action Toolbar
        // -------------------------------------------------------------------
        let top_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), 48.0));
        draw_doodle_panel(ui, top_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), 48.0),
            egui::Layout::left_to_right(egui::Align::Center),
            |ui| {
                ui.add_space(12.0);
                ui.label(RichText::new("🔬 DEBUG & INSPECT ENGINE").strong().size(13.0).color(theme.primary));
                ui.add_space(8.0);
                draw_highlighter_badge(ui, "● GDB PAUSED :1234", theme.success, theme.on_primary);

                ui.add_space(16.0);
                if ui.button("▶ Step Instruction (F7)").clicked() {
                    state.step_debugger();
                }
                if ui.button("⏩ Continue (F5)").clicked() {
                    state.add_log("GDB", "Resuming execution...", false);
                }
                if ui.button("⏹ Reset CPU").clicked() {
                    state.current_eip = 0x7C00;
                    state.init_mock_data();
                    state.add_log("GDB", "CPU Registers reset to BIOS Entry 0x7C00", false);
                }

                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("Target: x86 Real Mode (i8086)").size(11.0).color(theme.text_muted));
                });
            },
        );

        ui.add_space(8.0);

        // -------------------------------------------------------------------
        // 2. Main Debug 2x2 Grid
        // -------------------------------------------------------------------
        ui.horizontal(|ui| {
            let col_width = (ui.available_width() - 8.0) * 0.5;

            // ---------------------------------------------------------------
            // Left Column: CPU Registers + Stack Memory
            // ---------------------------------------------------------------
            ui.vertical(|ui| {
                let reg_height = (ui.available_height() * 0.52).max(180.0);
                let reg_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, reg_height));
                draw_doodle_panel(ui, reg_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, reg_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("📊 CPU REGISTERS & FLAGS").strong().size(12.0).color(theme.primary));
                            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                                ui.add_space(10.0);
                                ui.label(RichText::new("32-bit Context").size(10.0).color(theme.text_muted));
                            });
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ScrollArea::vertical()
                            .id_salt("registers_scroll")
                            .show(ui, |ui| {
                                ui.add_space(6.0);
                                egui::Grid::new("registers_grid")
                                    .num_columns(4)
                                    .spacing(Vec2::new(16.0, 8.0))
                                    .show(ui, |ui| {
                                        for (i, reg) in state.registers.iter().enumerate() {
                                            ui.label(RichText::new(&reg.name).strong().size(12.0).color(theme.text_primary));
                                            
                                            let val_str = format!("0x{:08X}", reg.value);
                                            if reg.changed {
                                                ui.label(RichText::new(&val_str).strong().monospace().size(12.0).color(theme.primary).background_color(Color32::from_rgba_premultiplied(0x00, 0xD4, 0xFF, 0x30)));
                                            } else {
                                                ui.label(RichText::new(&val_str).monospace().size(12.0).color(theme.text_primary));
                                            }

                                            if (i + 1) % 2 == 0 {
                                                ui.end_row();
                                            }
                                        }
                                    });
                            });
                    },
                );

                ui.add_space(8.0);

                // Stack Viewer
                let stack_height = ui.available_height().max(150.0);
                let stack_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, stack_height));
                draw_doodle_panel(ui, stack_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, stack_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("🥞 CALL STACK & FRAME").strong().size(12.0).color(theme.primary));
                            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                                ui.add_space(10.0);
                                ui.label(RichText::new("SP: 0x7C00").size(11.0).color(theme.text_muted));
                            });
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ScrollArea::vertical()
                            .id_salt("stack_scroll")
                            .show(ui, |ui| {
                                ui.add_space(4.0);
                                egui::Grid::new("stack_grid")
                                    .num_columns(3)
                                    .spacing(Vec2::new(20.0, 6.0))
                                    .show(ui, |ui| {
                                        ui.label(RichText::new("OFFSET").strong().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new("VALUE (HEX)").strong().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new("SYMBOL / ANNOTATION").strong().size(11.0).color(theme.text_muted));
                                        ui.end_row();

                                        for slot in &state.stack_slots {
                                            ui.label(RichText::new(&slot.offset).monospace().size(11.0).color(theme.text_primary));
                                            ui.label(RichText::new(format!("0x{:08X}", slot.value)).monospace().size(11.0).color(theme.primary));
                                            ui.label(RichText::new(slot.symbol.as_deref().unwrap_or("—")).italics().size(11.0).color(theme.text_muted));
                                            ui.end_row();
                                        }
                                    });
                            });
                    },
                );
            });

            ui.add_space(8.0);

            // ---------------------------------------------------------------
            // Right Column: Live Disassembly + Memory Inspector
            // ---------------------------------------------------------------
            ui.vertical(|ui| {
                let disasm_height = (ui.available_height() * 0.52).max(180.0);
                let disasm_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, disasm_height));
                draw_doodle_panel(ui, disasm_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, disasm_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("⚡ LIVE DISASSEMBLY STREAM").strong().size(12.0).color(theme.primary));
                            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                                ui.add_space(10.0);
                                ui.label(RichText::new(format!("EIP: 0x{:04X}", state.current_eip)).strong().monospace().size(11.0).color(theme.primary));
                            });
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ScrollArea::vertical()
                            .id_salt("disasm_scroll")
                            .show(ui, |ui| {
                                ui.add_space(6.0);
                                for line in &state.disassembly {
                                    ui.horizontal(|ui| {
                                        ui.add_space(8.0);
                                        // IP pointer arrow
                                        if line.address == state.current_eip {
                                            ui.label(RichText::new("▶").color(theme.primary).strong());
                                        } else {
                                            ui.label("  ");
                                        }

                                        // Address
                                        ui.label(RichText::new(format!("0x{:04X}:", line.address)).monospace().size(11.0).color(theme.text_muted));
                                        
                                        // Opcodes
                                        ui.label(RichText::new(format!("{:<8}", line.hex_bytes)).monospace().size(11.0).color(theme.secondary));

                                        // Instruction
                                        let inst_color = if line.address == state.current_eip {
                                            theme.primary
                                        } else {
                                            theme.text_primary
                                        };
                                        ui.label(RichText::new(&line.instruction).monospace().size(11.0).color(inst_color).strong());
                                    });
                                }
                            });
                    },
                );

                ui.add_space(8.0);

                // Memory Inspector
                let mem_height = ui.available_height().max(150.0);
                let mem_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, mem_height));
                draw_doodle_panel(ui, mem_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, mem_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("🔍 MEMORY INSPECTOR").strong().size(12.0).color(theme.primary));
                            ui.add_space(8.0);
                            ui.label("Address:");
                            ui.text_edit_singleline(&mut state.memory_address_input);
                            if ui.button("Read").clicked() {
                                state.add_log("Memory", &format!("Inspecting address {}", state.memory_address_input), false);
                            }
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ScrollArea::vertical()
                            .id_salt("mem_scroll")
                            .show(ui, |ui| {
                                ui.add_space(4.0);
                                let sample_rows = [
                                    ("0x00007C00", "FA 31 C0 8E D8 8E C0 8E  D0 BC 00 7C FB E8 0A 00", ".1........|....."),
                                    ("0x00007C10", "EB FE AC 08 C0 74 06 B4  0E CD 10 EB F5 C3 BE 20", ".....t......... "),
                                    ("0x00007C20", "7C E8 F0 FF F4 3E 3E 20  50 79 78 46 6F 72 67 65", "|....>> PyxForge"),
                                    ("0x00007DF0", "00 00 00 00 00 00 00 00  00 00 00 00 00 00 55 AA", "..............U."),
                                ];

                                for (addr, hex, ascii) in sample_rows {
                                    ui.horizontal(|ui| {
                                        ui.add_space(8.0);
                                        ui.label(RichText::new(addr).monospace().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new(hex).monospace().size(11.0).color(theme.primary));
                                        ui.label(RichText::new(format!("| {} |", ascii)).monospace().size(11.0).color(theme.secondary));
                                    });
                                }
                            });
                    },
                );
            });
        });
    });
}
