/**************************************************************************/
/*  spx_mgr_access.h                                                      */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#ifndef SPX_MGR_ACCESS_H
#define SPX_MGR_ACCESS_H

// Forward declaration to avoid circular dependency
class SpxEngine;
class SvgManager;
class SpxAudioBusPool;

/**
 * @file spx_mgr_access.h
 * @brief Unified accessor macros for all SPX manager instances
 *
 * This header provides convenient macros to access manager singletons throughout
 * the SPX module. All manager access macros are centralized here to avoid duplication.
 */

// SPX Manager access macros
#define inputMgr SpxEngine::get_singleton()->get_input()
#define audioMgr SpxEngine::get_singleton()->get_audio()
#define physicsMgr SpxEngine::get_singleton()->get_physics()
#define spriteMgr SpxEngine::get_singleton()->get_sprite()
#define uiMgr SpxEngine::get_singleton()->get_ui()
#define sceneMgr SpxEngine::get_singleton()->get_scene()
#define cameraMgr SpxEngine::get_singleton()->get_camera()
#define platformMgr SpxEngine::get_singleton()->get_platform()
#define resMgr SpxEngine::get_singleton()->get_res()
#define extMgr SpxEngine::get_singleton()->get_ext()
#define debugMgr SpxEngine::get_singleton()->get_debug()
#define navigationMgr SpxEngine::get_singleton()->get_navigation()
#define penMgr SpxEngine::get_singleton()->get_pen()
#define tilemapMgr SpxEngine::get_singleton()->get_tilemap()
#define tilemapparserMgr SpxEngine::get_singleton()->get_tilemapparser()

// Special Manager access macro
#define svgMgr SvgManager::get_singleton()
#define audioPool SpxAudioBusPool::get_singleton()
#define SPX_CALLBACK SpxEngine::get_singleton()->get_callbacks()

#endif // SPX_MGR_ACCESS_H
