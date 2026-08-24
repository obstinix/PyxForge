use egui::{RichText, Rounding, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, StitchTheme};

pub fn render_scaffold_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.vertical_centered(|ui| {
        ui.add_space(20.0);
        let card_width = 680.0;
        let card_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(card_width, 420.0));
        draw_doodle_panel(ui, card_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(8.0));

        ui.allocate_ui_with_layout(
            Vec2::new(card_width, 420.0),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(16.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("🚀 NEW OPERATING SYSTEM PROJECT WIZARD").strong().size(15.0).color(theme.primary));
                });
                ui.add_space(6.0);
                ui.separator();
                ui.add_space(12.0);

                // Project Name Input
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Project Name:").strong().size(13.0).color(theme.text_primary));
                    ui.add_space(8.0);
                    ui.text_edit_singleline(&mut state.scaffold_project_name);
                });

                ui.add_space(16.0);

                // Template Selection
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Select System Template:").strong().size(13.0).color(theme.text_primary));
                });

                ui.add_space(8.0);
                let templates = [
                    ("x86-realmode", "16-bit Real Mode MBR Bootloader", "NASM Assembly | BIOS 0x7C00 entry point"),
                    ("x86-protected", "32-bit Protected Mode Kernel", "C / GCC / LD | Multiboot2 header & GDT"),
                    ("x86_64-rust", "64-bit Rust Microkernel", "Freestanding Rustc | Long mode paging & interrupts"),
                    ("arm-cortex-m4", "ARM Bare-Metal Firmware", "QEMU LM3S6965 EVB board | Vector table"),
                ];

                for (id, title, desc) in templates {
                    ui.horizontal(|ui| {
                        ui.add_space(28.0);
                        let is_selected = state.scaffold_template == id;
                        if ui.radio(is_selected, RichText::new(title).strong().color(if is_selected { theme.primary } else { theme.text_primary })).clicked() {
                            state.scaffold_template = id.to_string();
                        }
                        ui.label(RichText::new(format!("— {}", desc)).size(11.0).color(theme.text_muted));
                    });
                    ui.add_space(6.0);
                }

                ui.add_space(16.0);

                // Output Location
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Location:").strong().size(13.0).color(theme.text_primary));
                    ui.add_space(8.0);
                    ui.label(RichText::new(state.scaffold_output_dir.display().to_string()).size(12.0).color(theme.secondary));
                    if ui.button("Browse...").clicked() {
                        if let Some(folder) = rfd::FileDialog::new().pick_folder() {
                            state.scaffold_output_dir = folder;
                        }
                    }
                });

                ui.add_space(24.0);

                // Create Button
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    if ui.button(RichText::new("✨ Initialize Project Workspace").strong().size(13.0).color(theme.on_primary)).clicked() {
                        let req = serde_json::json!({
                            "cmd": "init",
                            "projectRoot": state.scaffold_output_dir.to_string_lossy(),
                            "projectName": state.scaffold_project_name,
                            "template": state.scaffold_template,
                        });

                        match pyxforge_core::handle_request(&req.to_string()) {
                            Ok(res) => {
                                state.scaffold_status_msg = Some(format!("Project '{}' generated successfully!", state.scaffold_project_name));
                                state.add_log("Scaffold", &res, false);
                                state.scan_workspace();
                            }
                            Err(err) => {
                                state.scaffold_status_msg = Some(format!("Scaffold Error: {}", err));
                                state.add_log("Scaffold", &err, true);
                            }
                        }
                    }

                    if let Some(msg) = &state.scaffold_status_msg {
                        ui.add_space(12.0);
                        ui.label(RichText::new(msg).color(theme.success).strong());
                    }
                });
            },
        );
    });
}
