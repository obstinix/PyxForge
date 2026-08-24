use egui::{Color32, RichText, Rounding, ScrollArea, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, StitchTheme};

pub fn render_qemu_view(ui: &mut Ui, state: &mut AppState, theme: &StitchTheme) {
    ui.vertical(|ui| {
        // -------------------------------------------------------------------
        // 1. VM Status & Machine Configuration
        // -------------------------------------------------------------------
        let top_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(ui.available_width(), 120.0));
        draw_doodle_panel(ui, top_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

        ui.allocate_ui_with_layout(
            Vec2::new(ui.available_width(), 120.0),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("💻 QEMU HYPERVISOR & GUEST MACHINE CONTROL").strong().size(13.0).color(theme.primary));

                    let (status_badge, status_color) = if state.qemu_running {
                        (format!("● RUNNING (PID {})", state.qemu_pid.unwrap_or(4821)), theme.success)
                    } else {
                        ("○ VM STOPPED".to_string(), theme.text_muted)
                    };
                    draw_highlighter_badge(ui, &status_badge, status_color, theme.on_primary);

                    ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                        ui.add_space(12.0);
                        if state.qemu_running {
                            if ui.button(RichText::new("⏹ STOP VM").strong().color(theme.error)).clicked() {
                                state.qemu_running = false;
                                state.qemu_pid = None;
                                state.add_log("QEMU", "QEMU machine process terminated.", false);
                            }
                        } else {
                            if ui.button(RichText::new("▶ START VM").strong().color(theme.on_primary)).clicked() {
                                state.qemu_running = true;
                                state.qemu_pid = Some(5912);
                                state.add_log("QEMU", "QEMU launched: qemu-system-i386 -fda target/boot.bin -s -S", false);
                            }
                            if ui.button("🐞 Start (Debug Paused)").clicked() {
                                state.qemu_running = true;
                                state.qemu_pid = Some(5914);
                                state.add_log("QEMU", "QEMU paused on boot (GDB listening on :1234)", false);
                            }
                        }
                    });
                });

                ui.add_space(6.0);
                ui.separator();
                ui.add_space(6.0);

                // Machine params
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("Architecture:").strong().color(theme.text_muted));
                    ui.label(RichText::new("i386 (x86 32-bit / Real Mode)").color(theme.text_primary));
                    ui.add_space(16.0);
                    ui.label(RichText::new("RAM:").strong().color(theme.text_muted));
                    ui.label(RichText::new("128 MB").color(theme.text_primary));
                    ui.add_space(16.0);
                    ui.label(RichText::new("GDB Port:").strong().color(theme.text_muted));
                    ui.label(RichText::new(":1234").monospace().color(theme.primary));
                    ui.add_space(16.0);
                    ui.label(RichText::new("QMP Port:").strong().color(theme.text_muted));
                    ui.label(RichText::new(":4444").monospace().color(theme.secondary));
                });

                ui.add_space(4.0);
                ui.horizontal(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new("Boot Image: target/boot.bin | Display: stdio / curses").size(11.0).color(theme.text_muted));
                });
            },
        );

        ui.add_space(8.0);

        // -------------------------------------------------------------------
        // 2. Snapshot Manager + HMP Console
        // -------------------------------------------------------------------
        ui.horizontal(|ui| {
            let col_width = (ui.available_width() - 8.0) * 0.5;

            // ---------------------------------------------------------------
            // 2A. Snapshot Lifecycle Manager
            // ---------------------------------------------------------------
            ui.vertical(|ui| {
                let snap_height = ui.available_height().max(200.0);
                let snap_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, snap_height));
                draw_doodle_panel(ui, snap_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, snap_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("📸 SNAPSHOT LIFECYCLE MANAGER").strong().size(12.0).color(theme.primary));
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ui.add_space(6.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label("Tag:");
                            ui.text_edit_singleline(&mut state.snapshot_tag_input);
                            if ui.button("Save").clicked() {
                                let tag = state.snapshot_tag_input.clone();
                                state.snapshots.push(crate::state::SnapshotItem {
                                    tag: tag.clone(),
                                    vm_clock: "00:00:03.112".to_string(),
                                    date: "2026-08-24 15:30".to_string(),
                                    size: "256 KB".to_string(),
                                });
                                state.add_log("QMP", &format!("Snapshot '{}' saved successfully", tag), false);
                            }
                        });

                        ui.add_space(6.0);
                        ScrollArea::vertical()
                            .id_salt("snapshots_table_scroll")
                            .show(ui, |ui| {
                                ui.add_space(4.0);
                                egui::Grid::new("snapshots_grid")
                                    .num_columns(4)
                                    .spacing(Vec2::new(16.0, 6.0))
                                    .show(ui, |ui| {
                                        ui.label(RichText::new("TAG").strong().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new("VM CLOCK").strong().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new("TIMESTAMP").strong().size(11.0).color(theme.text_muted));
                                        ui.label(RichText::new("ACTIONS").strong().size(11.0).color(theme.text_muted));
                                        ui.end_row();

                                        let snaps = state.snapshots.clone();
                                        for snap in snaps {
                                            ui.label(RichText::new(&snap.tag).strong().monospace().size(11.0).color(theme.primary));
                                            ui.label(RichText::new(&snap.vm_clock).monospace().size(11.0).color(theme.text_primary));
                                            ui.label(RichText::new(&snap.date).size(11.0).color(theme.text_muted));
                                            
                                            ui.horizontal(|ui| {
                                                if ui.small_button("⏪ Restore").clicked() {
                                                    state.add_log("QMP", &format!("Restored VM to snapshot '{}'", snap.tag), false);
                                                }
                                            });
                                            ui.end_row();
                                        }
                                    });
                            });
                    },
                );
            });

            ui.add_space(8.0);

            // ---------------------------------------------------------------
            // 2B. Direct HMP Monitor Console
            // ---------------------------------------------------------------
            ui.vertical(|ui| {
                let hmp_height = ui.available_height().max(200.0);
                let hmp_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(col_width, hmp_height));
                draw_doodle_panel(ui, hmp_rect, Color32::from_rgb(0x05, 0x0F, 0x17), Stroke::new(1.0_f32, theme.outline), Rounding::same(6.0));

                ui.allocate_ui_with_layout(
                    Vec2::new(col_width, hmp_height),
                    egui::Layout::top_down(egui::Align::LEFT),
                    |ui| {
                        ui.add_space(8.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("📟 QEMU HMP MONITOR CONSOLE").strong().size(12.0).color(theme.primary));
                        });
                        ui.add_space(4.0);
                        ui.separator();

                        ui.add_space(6.0);
                        ui.horizontal(|ui| {
                            ui.add_space(10.0);
                            ui.label(RichText::new("(qemu)").monospace().color(theme.primary));
                            ui.text_edit_singleline(&mut state.hmp_input);
                            if ui.button("Execute").clicked() {
                                let cmd = state.hmp_input.clone();
                                state.hmp_output = format!(
                                    "(qemu) {}\nEAX=00000000 EBX=00007c00 ECX=00000020 EDX=00000080\nESI=00007c24 EDI=0000b800 EBP=00000000 ESP=00007c00\nEIP=00007c05 EFL=00000202 [-------] CPL=0 II=0 A20=1 SMM=0 HLT=0\nES =0000 00000000 0000ffff 00009300\nCS =0000 00000000 0000ffff 00009b00",
                                    cmd
                                );
                                state.add_log("HMP", &format!("Executed: {}", cmd), false);
                            }
                        });

                        ui.add_space(6.0);
                        ScrollArea::vertical()
                            .id_salt("hmp_output_scroll")
                            .show(ui, |ui| {
                                ui.add_space(4.0);
                                ui.horizontal(|ui| {
                                    ui.add_space(10.0);
                                    ui.label(RichText::new(&state.hmp_output).monospace().size(11.0).color(theme.primary));
                                });
                            });
                    },
                );
            });
        });
    });
}
