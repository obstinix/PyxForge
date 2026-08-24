mod state;
mod theme;
mod views;

use eframe::egui;
use egui::{RichText, Rounding, Stroke, Vec2};
use state::{ActiveScreen, AppState};
use theme::{apply_stitch_styles, draw_blueprint_grid, draw_doodle_panel, draw_highlighter_badge, draw_post_it_tab, StitchTheme};

struct PyxForgeApp {
    state: AppState,
    theme: StitchTheme,
}

impl PyxForgeApp {
    pub fn new(_cc: &eframe::CreationContext<'_>) -> Self {
        Self {
            state: AppState::default(),
            theme: StitchTheme::default(),
        }
    }
}

impl eframe::App for PyxForgeApp {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        // Apply Stitch Google UI Kit styling
        apply_stitch_styles(ctx, &self.theme);

        // -------------------------------------------------------------------
        // 1. TOP NAVIGATION & HEADER BAR
        // -------------------------------------------------------------------
        egui::TopBottomPanel::top("top_navbar")
            .exact_height(54.0)
            .show(ctx, |ui| {
                let rect = ui.available_rect_before_wrap();
                draw_doodle_panel(ui, rect, self.theme.surface_color, Stroke::new(1.0_f32, self.theme.outline), Rounding::same(0.0));

                ui.horizontal_centered(|ui| {
                    ui.add_space(12.0);

                    // Logo & Wordmark
                    ui.label(RichText::new("📐 PYXFORGE").strong().size(16.0).color(self.theme.primary));
                    draw_highlighter_badge(ui, "NATIVE RUST", self.theme.primary, self.theme.on_primary);

                    ui.add_space(16.0);
                    ui.separator();
                    ui.add_space(8.0);

                    // Navigation Tabs (Google Stitch Blueprint Tabs)
                    let nav_items = [
                        ("📁 Workspace", ActiveScreen::Workspace),
                        ("🔬 Debug & Inspect", ActiveScreen::DebugInspect),
                        ("🛠️ Build & Diag", ActiveScreen::BuildDiagnostics),
                        ("💻 QEMU Control", ActiveScreen::QemuControl),
                        ("🔍 Hex Explorer", ActiveScreen::HexExplorer),
                        ("🚀 New Project", ActiveScreen::NewProject),
                        ("🎨 Themes", ActiveScreen::ThemeGallery),
                    ];

                    for (title, screen) in nav_items {
                        let is_active = self.state.active_screen == screen;
                        if draw_post_it_tab(ui, title, is_active, &self.theme) {
                            self.state.active_screen = screen;
                        }
                        ui.add_space(2.0);
                    }

                    // Right-side Status & Quick Tools
                    ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                        ui.add_space(12.0);

                        // Core Status Pill
                        draw_highlighter_badge(ui, "● CORE RPC ONLINE", self.theme.success, self.theme.on_primary);

                        // QEMU Status Pill
                        if self.state.qemu_running {
                            draw_highlighter_badge(ui, "● QEMU RUNNING", self.theme.success, self.theme.on_primary);
                        } else {
                            draw_highlighter_badge(ui, "○ QEMU IDLE", self.theme.text_muted, self.theme.on_primary);
                        }

                        ui.label(RichText::new("PyxForge v0.1.0").size(11.0).color(self.theme.text_muted));
                    });
                });
            });

        // -------------------------------------------------------------------
        // 2. BOTTOM FOOTER STATUS BAR
        // -------------------------------------------------------------------
        egui::TopBottomPanel::bottom("bottom_statusbar")
            .exact_height(28.0)
            .show(ctx, |ui| {
                ui.horizontal_centered(|ui| {
                    ui.add_space(12.0);
                    ui.label(RichText::new(&self.state.status_message).size(11.0).color(self.theme.text_primary));

                    ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                        ui.add_space(12.0);
                        ui.label(RichText::new("🌿 branch: feat/native-rust-stitch-frontend").size(11.0).color(self.theme.primary));
                        ui.add_space(8.0);
                        ui.label(RichText::new("UTF-8 | LF | x86 Real Mode").size(11.0).color(self.theme.text_muted));
                    });
                });
            });

        // -------------------------------------------------------------------
        // 3. CENTRAL WORKSPACE AREA (Router)
        // -------------------------------------------------------------------
        egui::CentralPanel::default().show(ctx, |ui| {
            // Draw background blueprint graph paper grid
            let central_rect = ui.available_rect_before_wrap();
            draw_blueprint_grid(ui, central_rect, &self.theme);

            ui.add_space(8.0);

            match self.state.active_screen {
                ActiveScreen::Workspace => {
                    views::workspace::render_workspace_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::DebugInspect => {
                    views::debug::render_debug_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::BuildDiagnostics => {
                    views::build::render_build_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::QemuControl => {
                    views::qemu::render_qemu_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::HexExplorer => {
                    views::hex::render_hex_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::NewProject => {
                    views::scaffold::render_scaffold_view(ui, &mut self.state, &self.theme);
                }
                ActiveScreen::ThemeGallery => {
                    views::theme_gallery::render_theme_gallery_view(ui, &mut self.state, &mut self.theme);
                }
            }
        });
    }
}

fn main() -> Result<(), eframe::Error> {
    let native_options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title("PyxForge — Bare-Metal Developer Platform (Blueprint Doodle)")
            .with_inner_size(Vec2::new(1280.0, 840.0))
            .with_min_inner_size(Vec2::new(960.0, 600.0)),
        ..Default::default()
    };

    eframe::run_native(
        "PyxForge Desktop",
        native_options,
        Box::new(|cc| Ok(Box::new(PyxForgeApp::new(cc)))),
    )
}
