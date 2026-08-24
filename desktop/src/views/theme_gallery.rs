use egui::{RichText, Rounding, Stroke, Ui, Vec2};
use crate::state::AppState;
use crate::theme::{draw_doodle_panel, draw_highlighter_badge, draw_post_it_tab, StitchTheme, ThemeMode};

pub fn render_theme_gallery_view(
    ui: &mut Ui,
    _state: &mut AppState,
    theme: &mut StitchTheme,
) {
    ui.vertical_centered(|ui| {
        ui.add_space(16.0);
        let gallery_width = 720.0;
        let gallery_rect = egui::Rect::from_min_size(ui.cursor().min, Vec2::new(gallery_width, 480.0));
        draw_doodle_panel(ui, gallery_rect, theme.surface_color, Stroke::new(1.0_f32, theme.outline), Rounding::same(8.0));

        ui.allocate_ui_with_layout(
            Vec2::new(gallery_width, 480.0),
            egui::Layout::top_down(egui::Align::LEFT),
            |ui| {
                ui.add_space(16.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("🎨 GOOGLE STITCH DESIGN SYSTEM & THEME GALLERY").strong().size(15.0).color(theme.primary));
                });
                ui.add_space(6.0);
                ui.separator();
                ui.add_space(12.0);

                // Theme Mode Switcher
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Select Visual Paradigm:").strong().size(13.0).color(theme.text_primary));
                    ui.add_space(12.0);

                    if ui.selectable_label(theme.mode == ThemeMode::BlueprintDark, "📐 Blueprint Dark (Drafting Board)").clicked() {
                        *theme = StitchTheme::blueprint_dark();
                    }
                    if ui.selectable_label(theme.mode == ThemeMode::WarmPaperDoodle, "📜 Warm Paper Doodle (Ink & Paper)").clicked() {
                        *theme = StitchTheme::warm_paper();
                    }
                });

                ui.add_space(16.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Design System Color Tokens:").strong().size(12.0).color(theme.text_muted));
                });

                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    let swatches = [
                        ("Primary Cyan", theme.primary),
                        ("Surface Dark", theme.surface_color),
                        ("Secondary Blue", theme.secondary),
                        ("Text Light", theme.text_primary),
                        ("Warning Amber", theme.warning),
                        ("Success Lime", theme.success),
                        ("Error Red", theme.error),
                    ];

                    for (name, color) in swatches {
                        ui.vertical(|ui| {
                            let (rect, _) = ui.allocate_exact_size(Vec2::new(60.0, 32.0), egui::Sense::hover());
                            ui.painter().rect_filled(rect, Rounding::same(4.0), color);
                            ui.painter().rect_stroke(rect, Rounding::same(4.0), Stroke::new(1.0_f32, theme.outline));
                            ui.label(RichText::new(name).size(10.0).color(theme.text_muted));
                        });
                        ui.add_space(8.0);
                    }
                });

                ui.add_space(20.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Tactile Component Demonstrations:").strong().size(12.0).color(theme.text_muted));
                });

                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    draw_highlighter_badge(ui, "HIGHLIGHTER BADGE", theme.primary, theme.on_primary);
                    ui.add_space(8.0);
                    draw_highlighter_badge(ui, "SUCCESS 0xAA55", theme.success, theme.on_primary);
                    ui.add_space(8.0);
                    draw_highlighter_badge(ui, "ALERT WARNING", theme.warning, theme.on_primary);
                    ui.add_space(8.0);
                    draw_post_it_tab(ui, "Post-it Tab Demo", true, theme);
                });

                ui.add_space(16.0);
                ui.horizontal(|ui| {
                    ui.add_space(20.0);
                    ui.label(RichText::new("Typography:").strong().size(12.0).color(theme.text_muted));
                    ui.label(RichText::new("JetBrains Mono (Technical/Hex)").monospace().size(12.0).color(theme.primary));
                    ui.label(RichText::new("• Bricolage Grotesque / Unbounded").size(12.0).color(theme.text_primary));
                });
            },
        );
    });
}
