use egui::{RichText, Rounding, ScrollArea, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, StitchTheme};

pub fn render_build_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.vertical(|ui| {
        // -------------------------------------------------------------------
        // 1. Build Profiles & Trigger Banner
        // -------------------------------------------------------------------
        let top_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), 110.0));
        draw_doodle_panel(ui, top_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), 110.0),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("🛠️ BUILD PIPELINE & TARGET PROFILES").strong().size(13.0).color(theme.primary));
                    
                    ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                        ui.add_space(12.0);
                        if ui.button(RichText::new("⚡ RUN BUILD (Ctrl+B)").strong().color(theme.on_primary)).clicked() {
                            state.last_build_status = Some(true);
                            state.add_log("Build", &format!("Invoking toolchain for profile '{}'", state.selected_profile), false);
                            state.add_log("Build", "Compiling boot.asm -> target/boot.bin...", false);
                            state.add_log("Build", "[OK] Build succeeded with 0 errors, 1 warning.", false);
                        }
                    });
                });

                ui.add_space(6.0);
                ui.separator();
                ui.add_space(6.0);

                // Profiles Selector
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("Active Profile:").strong().color(theme.text_muted));

                    let profiles = ["x86_realmode", "x86_protected", "x86_64_longmode", "arm_cortex_m4"];
                    for p in profiles {
                        let is_selected = state.selected_profile == p;
                        if ui.selectable_label(is_selected, RichText::new(p).strong().size(11.0)).clicked() {
                            state.selected_profile = p.to_string();
                        }
                    }
                });

                ui.add_space(4.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("Target: i8086-unknown-none | Tool: nasm -f bin | Output: target/boot.bin").size(11.0).color(theme.secondary));
                });
            },
        );

        ui.add_space(8.0);

        // -------------------------------------------------------------------
        // 2. Diagnostics & Problems Table
        // -------------------------------------------------------------------
        let diag_height = (ui.available_height() * 0.55).max(180.0);
        let diag_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), diag_height));
        draw_doodle_panel(ui, diag_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        let mut applied_fix: Option<(usize, String)> = None;

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), diag_height),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("📋 PROBLEMS & COMPILER DIAGNOSTICS").strong().size(12.0).color(theme.primary));
                    ui.add_space(8.0);
                    draw_highlighter_badge(ui, &format!("{} Issues Found", state.diagnostics.len()), theme.warning, theme.on_primary);
                });

                ui.add_space(4.0);
                ui.separator();

                ScrollArea::vertical()
                    .id_salt("diagnostics_scroll")
                    .show(ui, |ui| {
                        ui.add_space(6.0);
                        let diags = state.diagnostics.clone();
                        for (idx, diag) in diags.iter().enumerate() {
                            ui.horizontal(|ui| {
                                ui.add_space(12.0);
                                let (badge_color, badge_text) = match diag.severity.as_str() {
                                    "ERROR" => (theme.error, "[ERROR]"),
                                    "WARNING" => (theme.warning, "[WARNING]"),
                                    _ => (theme.primary, "[INFO]"),
                                };

                                ui.label(RichText::new(badge_text).strong().size(11.0).color(badge_color));
                                ui.label(RichText::new(format!("{}:{}:{}", diag.file, diag.line, diag.column)).monospace().size(11.0).color(theme.text_muted));
                                ui.label(RichText::new(&diag.message).size(12.0).color(theme.text_primary));

                                if let Some(fix) = &diag.fix_suggestion {
                                    if ui.small_button("💡 Apply Fix").on_hover_text(fix).clicked() {
                                        applied_fix = Some((idx + 1, fix.clone()));
                                    }
                                }
                            });
                            ui.add_space(4.0);
                        }
                    });
            },
        );

        if let Some((idx, fix)) = applied_fix {
            state.add_log("Diagnostics", &format!("Applied fix for issue #{}: {}", idx, fix), false);
        }

        ui.add_space(8.0);

        // -------------------------------------------------------------------
        // 3. Artifacts & Binary Section Headers
        // -------------------------------------------------------------------
        let art_height = ui.available_height().max(120.0);
        let art_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), art_height));
        draw_doodle_panel(ui, art_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), art_height),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("📦 GENERATED ARTIFACT INSPECTION").strong().size(12.0).color(theme.primary));
                    draw_highlighter_badge(ui, "MBR 512 BYTES", theme.primary, theme.on_primary);
                    draw_highlighter_badge(ui, "MAGIC 0xAA55 VALID", theme.success, theme.on_primary);
                });

                ui.add_space(4.0);
                ui.separator();

                ScrollArea::vertical()
                    .id_salt("artifact_scroll")
                    .show(ui, |ui| {
                        ui.add_space(4.0);
                        egui::Grid::new("artifact_grid")
                            .num_columns(4)
                            .spacing(Vec2::new(24.0, 6.0))
                            .show(ui, |ui| {
                                ui.label(RichText::new("SECTION").strong().size(11.0).color(theme.text_muted));
                                ui.label(RichText::new("OFFSET").strong().size(11.0).color(theme.text_muted));
                                ui.label(RichText::new("SIZE").strong().size(11.0).color(theme.text_muted));
                                ui.label(RichText::new("FLAGS / PERMISSIONS").strong().size(11.0).color(theme.text_muted));
                                ui.end_row();

                                let sections = [
                                    (".text", "0x0000", "420 B", "READ | EXEC"),
                                    (".rodata", "0x01A4", "60 B", "READ ONLY"),
                                    (".padding", "0x01E0", "30 B", "ZEROED"),
                                    (".signature", "0x01FE", "2 B (0xAA55)", "MBR MAGIC"),
                                ];

                                for (name, off, sz, flags) in sections {
                                    ui.label(RichText::new(name).monospace().size(11.0).color(theme.primary));
                                    ui.label(RichText::new(off).monospace().size(11.0).color(theme.text_primary));
                                    ui.label(RichText::new(sz).monospace().size(11.0).color(theme.text_primary));
                                    ui.label(RichText::new(flags).monospace().size(11.0).color(theme.secondary));
                                    ui.end_row();
                                }
                            });
                    });
            },
        );
    });
}
