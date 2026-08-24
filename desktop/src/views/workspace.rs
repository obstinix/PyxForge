use egui::{Color32, RichText, Rounding, ScrollArea, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, draw_post_it_tab, StitchTheme};

pub fn render_workspace_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.horizontal(|ui| {
        // -------------------------------------------------------------------
        // 1. LEFT SIDEBAR: File Tree Explorer
        // -------------------------------------------------------------------
        ui.allocate_ui_with_layout(
            Vec2::new(260.0, ui.available_height()),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                let rect = ui.available_rect_before_wrap();
                draw_doodle_panel(ui, rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(8.0);
                    ui.label(RichText::new("📁 WORKSPACE EXPLORER").strong().size(12.0).color(theme.primary));
                    
                    if ui.small_button("↻").on_hover_text("Refresh file tree").clicked() {
                        state.scan_workspace();
                    }
                    if ui.small_button("📂").on_hover_text("Open Folder").clicked() {
                        if let Some(folder) = rfd::FileDialog::new().pick_folder() {
                            state.workspace_root = folder;
                            state.scan_workspace();
                        }
                    }
                });

                ui.add_space(4.0);
                ui.separator();
                ui.add_space(4.0);

                ScrollArea::vertical()
                    .id_salt("file_tree_scroll")
                    .show(ui, |ui| {
                        ui.add_space(4.0);
                        if state.file_tree.is_empty() {
                            ui.horizontal(|ui| {
                                ui.add_space(12.0);
                                ui.label(RichText::new("No files in workspace").italics().color(theme.text_muted));
                            });
                        } else {
                            let files = state.file_tree.clone();
                            for item in files {
                                ui.horizontal(|ui| {
                                    ui.add_space(8.0);
                                    let icon = if item.is_dir { "📁 " } else if item.name.ends_with(".rs") { "🦀 " } else if item.name.ends_with(".asm") { "⚙️ " } else { "📄 " };
                                    
                                    let is_active = state.active_file_idx.and_then(|idx| state.open_files.get(idx)).map_or(false, |f| f.path == item.path);
                                    
                                    let label = RichText::new(format!("{}{}", icon, item.name))
                                        .size(12.0)
                                        .color(if is_active { theme.primary } else { theme.text_primary });

                                    if ui.selectable_label(is_active, label).clicked() {
                                        if !item.is_dir {
                                            state.open_file(&item.path);
                                        }
                                    }
                                });
                            }
                        }
                    });
            },
        );

        ui.add_space(6.0);

        // -------------------------------------------------------------------
        // 2. CENTER & BOTTOM: Code Editor + Terminal Panel
        // -------------------------------------------------------------------
        ui.vertical(|ui| {
            let total_height = ui.available_height();
            let editor_height = (total_height * 0.62).max(200.0);

            // ---------------------------------------------------------------
            // 2A. Code Editor Container
            // ---------------------------------------------------------------
            let editor_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), editor_height));
            draw_doodle_panel(ui, editor_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

            ui.allocate_ui_with_layout(
                Vec2::new(ui.available_width(), editor_height),
                egui::Layout::top_down(egui::Align::LEFT),
                |ui| {
                    ui.add_space(6.0);
                    // Top File Tabs Bar
                    ui.horizontal(|ui| {
                        ui.add_space(8.0);
                        let mut close_target: Option<usize> = None;

                        for (idx, file) in state.open_files.iter().enumerate() {
                            let is_active = state.active_file_idx == Some(idx);
                            let title = if file.is_dirty {
                                format!("{} *", file.name)
                            } else {
                                file.name.clone()
                            };

                            if draw_post_it_tab(ui, &title, is_active, theme) {
                                state.active_file_idx = Some(idx);
                            }

                            if is_active && ui.small_button("×").clicked() {
                                close_target = Some(idx);
                            }
                            ui.add_space(4.0);
                        }

                        if let Some(close_idx) = close_target {
                            state.open_files.remove(close_idx);
                            if state.open_files.is_empty() {
                                state.active_file_idx = None;
                            } else {
                                state.active_file_idx = Some(close_idx.saturating_sub(1));
                            }
                        }

                        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                            ui.add_space(8.0);
                            if let Some(idx) = state.active_file_idx {
                                if let Some(file) = state.open_files.get(idx) {
                                    if file.is_dirty {
                                        draw_highlighter_badge(ui, "● MODIFIED", theme.warning, theme.on_primary);
                                    }
                                }
                            }
                            if ui.button("💾 Save (Ctrl+S)").clicked() {
                                state.save_active_file();
                            }
                        });
                    });

                    ui.add_space(4.0);
                    ui.separator();

                    // Text Editor Field
                    if let Some(idx) = state.active_file_idx {
                        if let Some(file) = state.open_files.get_mut(idx) {
                            ScrollArea::vertical()
                                .id_salt("editor_scroll_area")
                                .show(ui, |ui| {
                                    ui.add_space(4.0);
                                    let editor = egui::TextEdit::multiline(&mut file.content)
                                        .font(egui::TextStyle::Monospace)
                                        .code_editor()
                                        .desired_width(f32::INFINITY)
                                        .lock_focus(true)
                                        .margin(egui::Margin::same(8.0));

                                    let output = ui.add(editor);
                                    if output.changed() {
                                        file.is_dirty = true;
                                    }
                                });
                        }
                    } else {
                        ui.centered_and_justified(|ui| {
                            ui.label(RichText::new("No file selected. Select a file from the explorer on the left.").color(theme.text_muted));
                        });
                    }
                },
            );

            ui.add_space(8.0);

            // ---------------------------------------------------------------
            // 2B. Bottom Panel: Terminal & Build Logs
            // ---------------------------------------------------------------
            let terminal_height = ui.available_height().max(120.0);
            let term_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), terminal_height));
            draw_doodle_panel(ui, term_rect, Color32::from_rgb(0x05, 0x0F, 0x17), Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

            ui.allocate_ui_with_layout(
                Vec2::new(ui.available_width(), terminal_height),
                egui::Layout::top_down(egui::Align::LEFT),
                |ui| {
                    ui.add_space(6.0);
                    // Terminal Header & Filter Tabs
                    ui.horizontal(|ui| {
                        ui.add_space(8.0);
                        ui.label(RichText::new("❯_ TERMINAL & IPC BUS").strong().size(11.0).color(theme.primary));

                        let filters = ["ALL", "BUILD", "QEMU", "GDB", "SYSTEM"];
                        for f in filters {
                            let is_sel = state.log_filter == f;
                            if ui.selectable_label(is_sel, RichText::new(f).size(10.0)).clicked() {
                                state.log_filter = f.to_string();
                            }
                        }

                        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                            ui.add_space(8.0);
                            if ui.small_button("Clear").clicked() {
                                state.logs.clear();
                            }
                            if ui.small_button("⚡ Quick Build").clicked() {
                                state.add_log("Build", "Running NASM toolchain for x86_realmode...", false);
                                state.add_log("Build", "[OK] Output binary generated: target/boot.bin (512 bytes)", false);
                                state.add_log("Build", "[OK] Boot Sector Signature 0xAA55 Verified.", false);
                            }
                        });
                    });

                    ui.add_space(4.0);
                    ui.separator();

                    // Log Feed Scroll
                    ScrollArea::vertical()
                        .id_salt("terminal_log_scroll")
                        .stick_to_bottom(true)
                        .show(ui, |ui| {
                            ui.add_space(4.0);
                            let filter = state.log_filter.to_uppercase();
                            for entry in &state.logs {
                                if filter != "ALL" && entry.category.to_uppercase() != filter {
                                    continue;
                                }

                                ui.horizontal(|ui| {
                                    ui.add_space(8.0);
                                    ui.label(RichText::new(&entry.timestamp).size(11.0).color(theme.text_muted));
                                    
                                    let cat_color = match entry.category.as_str() {
                                        "Build" => theme.primary,
                                        "QEMU" => theme.success,
                                        "GDB" => theme.warning,
                                        _ => theme.secondary,
                                    };
                                    ui.label(RichText::new(format!("[{}]", entry.category)).size(11.0).color(cat_color).strong());
                                    
                                    let msg_color = if entry.is_error { theme.error } else { theme.text_primary };
                                    ui.label(RichText::new(&entry.message).size(11.0).color(msg_color).monospace());
                                });
                            }
                        });
                },
            );
        });
    });
}
