use egui::{Color32, Pos2, Rect, Rounding, Stroke, Ui, Vec2};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ThemeMode {
    BlueprintDark,
    WarmPaperDoodle,
}

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct StitchTheme {
    pub mode: ThemeMode,
    pub bg_color: Color32,
    pub surface_color: Color32,
    pub surface_high: Color32,
    pub surface_highest: Color32,
    pub primary: Color32,
    pub on_primary: Color32,
    pub secondary: Color32,
    pub on_secondary: Color32,
    pub text_primary: Color32,
    pub text_muted: Color32,
    pub outline: Color32,
    pub outline_variant: Color32,
    pub error: Color32,
    pub success: Color32,
    pub warning: Color32,
    pub grid_line: Color32,
}

impl Default for StitchTheme {
    fn default() -> Self {
        Self::blueprint_dark()
    }
}

impl StitchTheme {
    pub fn blueprint_dark() -> Self {
        Self {
            mode: ThemeMode::BlueprintDark,
            bg_color: Color32::from_rgb(0x07, 0x15, 0x1F),         // #07151f
            surface_color: Color32::from_rgb(0x13, 0x21, 0x2C),    // #13212c
            surface_high: Color32::from_rgb(0x1E, 0x2B, 0x37),     // #1e2b37
            surface_highest: Color32::from_rgb(0x29, 0x36, 0x42),  // #293642
            primary: Color32::from_rgb(0x00, 0xD4, 0xFF),          // #00d4ff Highlighter Cyan
            on_primary: Color32::from_rgb(0x00, 0x36, 0x42),       // #003642
            secondary: Color32::from_rgb(0x1F, 0x47, 0x7B),        // #1f477b
            on_secondary: Color32::from_rgb(0xA7, 0xC8, 0xFF),     // #a7c8ff
            text_primary: Color32::from_rgb(0xD6, 0xE4, 0xF4),     // #d6e4f4
            text_muted: Color32::from_rgb(0x85, 0x93, 0x98),       // #859398
            outline: Color32::from_rgb(0x3C, 0x49, 0x4E),          // #3c494e
            outline_variant: Color32::from_rgb(0x1F, 0x34, 0x48),  // #1f3448
            error: Color32::from_rgb(0xFF, 0xB4, 0xAB),            // #ffb4ab
            success: Color32::from_rgb(0x34, 0xD3, 0x99),          // #34d399
            warning: Color32::from_rgb(0xFB, 0xBF, 0x24),          // #fbbf24
            grid_line: Color32::from_rgba_premultiplied(0x00, 0x67, 0x7E, 0x30),
        }
    }

    pub fn warm_paper() -> Self {
        Self {
            mode: ThemeMode::WarmPaperDoodle,
            bg_color: Color32::from_rgb(0xFA, 0xF9, 0xF6),         // #FAF9F6 Warm Paper
            surface_color: Color32::from_rgb(0xEE, 0xEE, 0xEE),    // #eeeeee
            surface_high: Color32::from_rgb(0xE2, 0xE2, 0xE2),     // #e2e2e2
            surface_highest: Color32::from_rgb(0xD5, 0xD5, 0xD5),  // #d5d5d5
            primary: Color32::from_rgb(0x00, 0x98, 0xB8),          // Deep Cyan Ink
            on_primary: Color32::from_rgb(0xFF, 0xFF, 0xFF),
            secondary: Color32::from_rgb(0x5E, 0x5F, 0x5D),
            on_secondary: Color32::from_rgb(0xFA, 0xF9, 0xF6),
            text_primary: Color32::from_rgb(0x1A, 0x1C, 0x1C),     // Technical Black Ink
            text_muted: Color32::from_rgb(0x7E, 0x75, 0x76),
            outline: Color32::from_rgb(0xCF, 0xC4, 0xD5),
            outline_variant: Color32::from_rgb(0xDF, 0xD4, 0xD5),
            error: Color32::from_rgb(0xBA, 0x1A, 0x1A),
            success: Color32::from_rgb(0x16, 0xA3, 0x4A),
            warning: Color32::from_rgb(0xD9, 0x77, 0x06),
            grid_line: Color32::from_rgba_premultiplied(0x80, 0x90, 0xA0, 0x25),
        }
    }
}

/// Applies the Stitch Google UI Kit styling to the egui Context.
pub fn apply_stitch_styles(ctx: &egui::Context, theme: &StitchTheme) {
    let mut visuals = if theme.mode == ThemeMode::BlueprintDark {
        egui::Visuals::dark()
    } else {
        egui::Visuals::light()
    };

    visuals.panel_fill = theme.bg_color;
    visuals.window_fill = theme.surface_color;
    visuals.override_text_color = Some(theme.text_primary);
    
    // Widgets inactive
    visuals.widgets.inactive.bg_fill = theme.surface_color;
    visuals.widgets.inactive.fg_stroke = Stroke::new(1.0_f32, theme.text_primary);
    visuals.widgets.inactive.bg_stroke = Stroke::new(1.0_f32, theme.outline);
    visuals.widgets.inactive.rounding = Rounding::same(4.0);

    // Widgets hovered
    visuals.widgets.hovered.bg_fill = theme.surface_high;
    visuals.widgets.hovered.fg_stroke = Stroke::new(1.5_f32, theme.primary);
    visuals.widgets.hovered.bg_stroke = Stroke::new(1.5_f32, theme.primary);
    visuals.widgets.hovered.rounding = Rounding::same(4.0);

    // Widgets active / clicked
    visuals.widgets.active.bg_fill = theme.primary;
    visuals.widgets.active.fg_stroke = Stroke::new(1.5_f32, theme.on_primary);
    visuals.widgets.active.bg_stroke = Stroke::new(2.0_f32, theme.primary);
    visuals.widgets.active.rounding = Rounding::same(4.0);

    // Selection
    visuals.selection.bg_fill = Color32::from_rgba_premultiplied(theme.primary.r(), theme.primary.g(), theme.primary.b(), 0x50);
    visuals.selection.stroke = Stroke::new(1.0_f32, theme.primary);

    ctx.set_visuals(visuals);

    // Setup typography fonts
    let mut style = (*ctx.style()).clone();
    style.spacing.item_spacing = Vec2::new(8.0, 6.0);
    style.spacing.window_margin = egui::Margin::same(12.0);
    ctx.set_style(style);
}

// ---------------------------------------------------------------------------
// Tactile Doodle / Blueprint Custom Painter Helpers
// ---------------------------------------------------------------------------

/// Draws the blueprint graph paper grid with dots.
pub fn draw_blueprint_grid(ui: &Ui, rect: Rect, theme: &StitchTheme) {
    let painter = ui.painter();
    let step = 24.0;

    let min_x = (rect.min.x / step).floor() * step;
    let max_x = rect.max.x;
    let min_y = (rect.min.y / step).floor() * step;
    let max_y = rect.max.y;

    // Grid dots / lines
    let mut x = min_x;
    while x <= max_x {
        let mut y = min_y;
        while y <= max_y {
            painter.circle_filled(Pos2::new(x, y), 1.0, theme.grid_line);
            y += step;
        }
        x += step;
    }
}

/// Draws a hand-drawn tactile doodle panel box with subtle offset shadow stroke.
pub fn draw_doodle_panel(
    ui: &Ui,
    rect: Rect,
    bg_fill: Color32,
    border_stroke: Stroke,
    rounding: Rounding,
) {
    let painter = ui.painter();

    // 1. Offset Shadow Stroke (tactile hand-drawn elevation)
    let shadow_rect = rect.translate(Vec2::new(3.0, 3.0));
    painter.rect_stroke(
        shadow_rect,
        rounding,
        Stroke::new(1.5_f32, Color32::from_rgba_premultiplied(0x00, 0x10, 0x20, 0x90)),
    );

    // 2. Main Box Fill
    painter.rect_filled(rect, rounding, bg_fill);

    // 3. Wobbly Hand-Drawn Primary Border
    painter.rect_stroke(rect, rounding, border_stroke);

    // 4. Subtle corner ticks (mimicking pencil drafting box overshoot)
    let tick_len = 3.0;
    // Top-left
    painter.line_segment(
        [Pos2::new(rect.min.x - tick_len, rect.min.y), Pos2::new(rect.min.x + tick_len, rect.min.y)],
        border_stroke,
    );
    painter.line_segment(
        [Pos2::new(rect.min.x, rect.min.y - tick_len), Pos2::new(rect.min.x, rect.min.y + tick_len)],
        border_stroke,
    );
    // Bottom-right
    painter.line_segment(
        [Pos2::new(rect.max.x - tick_len, rect.max.y), Pos2::new(rect.max.x + tick_len, rect.max.y)],
        border_stroke,
    );
    painter.line_segment(
        [Pos2::new(rect.max.x, rect.max.y - tick_len), Pos2::new(rect.max.x, rect.max.y + tick_len)],
        border_stroke,
    );
}

/// Draws a tactile post-it note tab with semi-transparent tape on top.
pub fn draw_post_it_tab(
    ui: &mut Ui,
    title: &str,
    is_active: bool,
    theme: &StitchTheme,
) -> bool {
    let text = format!(" {} ", title);
    let button = if is_active {
        egui::Button::new(
            egui::RichText::new(&text)
                .color(theme.on_primary)
                .strong()
                .size(13.0),
        )
        .fill(theme.primary)
        .rounding(Rounding::same(4.0))
        .stroke(Stroke::new(1.5_f32, theme.primary))
    } else {
        egui::Button::new(
            egui::RichText::new(&text)
                .color(theme.text_primary)
                .size(13.0),
        )
        .fill(theme.surface_color)
        .rounding(Rounding::same(4.0))
        .stroke(Stroke::new(1.0_f32, theme.outline))
    };

    let response = ui.add(button);

    // Draw small tape badge at top if active
    if is_active {
        let rect = response.rect;
        let tape_rect = Rect::from_min_size(
            Pos2::new(rect.center().x - 12.0, rect.min.y - 2.0),
            Vec2::new(24.0, 5.0),
        );
        ui.painter().rect_filled(
            tape_rect,
            Rounding::same(1.0),
            Color32::from_rgba_premultiplied(0xFF, 0xFF, 0xFF, 0x80),
        );
    }

    response.clicked()
}

/// Draws a felt-tip highlighter badge.
pub fn draw_highlighter_badge(ui: &mut Ui, label: &str, color: Color32, text_color: Color32) {
    let font_id = egui::FontId::monospace(11.0);
    let galley = ui.painter().layout_no_wrap(label.to_string(), font_id, text_color);
    let padding = Vec2::new(8.0, 3.0);
    let size = galley.size() + padding * 2.0;

    let (rect, _) = ui.allocate_exact_size(size, egui::Sense::hover());
    
    // Highlighter ink background
    ui.painter().rect_filled(
        rect,
        Rounding::same(3.0),
        Color32::from_rgba_premultiplied(color.r(), color.g(), color.b(), 0x35),
    );
    ui.painter().rect_stroke(rect, Rounding::same(3.0), Stroke::new(1.0_f32, color));
    
    ui.painter().galley(rect.min + padding, galley, text_color);
}
