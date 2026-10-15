extends Control

const BASE_PANEL_HEIGHT := 106.0
const BASE_QUESTION_HEIGHT := 20.0
const QUESTION_BOTTOM := 42.0

@onready var ask_panel: Control = $MF
@onready var ask_body: Control = $MF/Frame/AskBody
@onready var label_ask: Label = $MF/Frame/AskBody/LabelAsk
@onready var input: LineEdit = $M/Input

var _layout_update_queued := false
var _ui_font: Font


func _ready() -> void:
	_install_ui_font()
	get_viewport().size_changed.connect(_queue_layout_update)
	ask_body.visibility_changed.connect(_queue_layout_update)
	label_ask.minimum_size_changed.connect(_queue_layout_update)
	_queue_layout_update()


func _install_ui_font() -> void:
	_ui_font = label_ask.get_theme_font("font").duplicate(true) as Font
	_set_font_oversampling(_ui_font, 2.0)
	label_ask.add_theme_font_override("font", _ui_font)
	input.add_theme_font_override("font", _ui_font)


func _set_font_oversampling(font: Font, value: float) -> void:
	if font == null:
		return
	if font is FontFile:
		(font as FontFile).oversampling = value
	elif font is FontVariation:
		_set_font_oversampling((font as FontVariation).base_font, value)
	for fallback in font.fallbacks:
		_set_font_oversampling(fallback, value)


func _queue_layout_update() -> void:
	if _layout_update_queued:
		return
	_layout_update_queued = true
	call_deferred("_apply_screen_layout")


func _apply_screen_layout() -> void:
	_layout_update_queued = false
	var stretch_transform := get_viewport().get_stretch_transform()
	var canvas_scale := stretch_transform.get_scale().abs()
	canvas_scale.x = maxf(canvas_scale.x, 0.001)
	canvas_scale.y = maxf(canvas_scale.y, 0.001)
	var content_size: Vector2
	if get_window().content_scale_mode == Window.CONTENT_SCALE_MODE_DISABLED:
		content_size = get_viewport().get_visible_rect().size
	else:
		content_size = Vector2(get_window().content_scale_size) * canvas_scale
	content_size = content_size.round()

	anchor_left = 0.0
	anchor_top = 0.0
	anchor_right = 0.0
	anchor_bottom = 0.0
	offset_left = 0.0
	offset_top = 0.0
	offset_right = content_size.x
	offset_bottom = content_size.y

	pivot_offset = Vector2.ZERO
	scale = Vector2(1.0 / canvas_scale.x, 1.0 / canvas_scale.y)

	var question_height := 0.0
	if ask_body.visible:
		question_height = ceilf(label_ask.get_combined_minimum_size().y)
	var extra_height := maxf(question_height - BASE_QUESTION_HEIGHT, 0.0)
	ask_panel.offset_top = -(BASE_PANEL_HEIGHT + extra_height)
	label_ask.offset_bottom = QUESTION_BOTTOM + extra_height
