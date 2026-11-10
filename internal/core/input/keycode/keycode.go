/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package keycode

// Low-level key codes used by the engine runtime.
// Higher-level packages should re-export the stable subset they want to expose.
const (
	KeyNone         = 0
	KeySpecial      = 4194304
	KeyEscape       = 4194305
	KeyTab          = 4194306
	KeyBacktab      = 4194307
	KeyBackspace    = 4194308
	KeyEnter        = 4194309
	KeyKPEnter      = 4194310
	KeyInsert       = 4194311
	KeyDelete       = 4194312
	KeyPause        = 4194313
	KeyPrint        = 4194314
	KeySysReq       = 4194315
	KeyClear        = 4194316
	KeyHome         = 4194317
	KeyEnd          = 4194318
	KeyLeft         = 4194319
	KeyUp           = 4194320
	KeyRight        = 4194321
	KeyDown         = 4194322
	KeyPageUp       = 4194323
	KeyPageDown     = 4194324
	KeyShift        = 4194325
	KeyCtrl         = 4194326
	KeyMeta         = 4194327
	KeyCmdOrCtrl    = 4194327
	KeyAlt          = 4194328
	KeyCapsLock     = 4194329
	KeyNumLock      = 4194330
	KeyScrollLock   = 4194331
	KeyF1           = 4194332
	KeyF2           = 4194333
	KeyF3           = 4194334
	KeyF4           = 4194335
	KeyF5           = 4194336
	KeyF6           = 4194337
	KeyF7           = 4194338
	KeyF8           = 4194339
	KeyF9           = 4194340
	KeyF10          = 4194341
	KeyF11          = 4194342
	KeyF12          = 4194343
	KeyF13          = 4194344
	KeyF14          = 4194345
	KeyF15          = 4194346
	KeyF16          = 4194347
	KeyF17          = 4194348
	KeyF18          = 4194349
	KeyF19          = 4194350
	KeyF20          = 4194351
	KeyF21          = 4194352
	KeyF22          = 4194353
	KeyF23          = 4194354
	KeyF24          = 4194355
	KeyF25          = 4194356
	KeyF26          = 4194357
	KeyF27          = 4194358
	KeyF28          = 4194359
	KeyF29          = 4194360
	KeyF30          = 4194361
	KeyF31          = 4194362
	KeyF32          = 4194363
	KeyF33          = 4194364
	KeyF34          = 4194365
	KeyF35          = 4194366
	KeyKPMultiply   = 4194433
	KeyKPDivide     = 4194434
	KeyKPSubtract   = 4194435
	KeyKPPeriod     = 4194436
	KeyKPAdd        = 4194437
	KeyKP0          = 4194438
	KeyKP1          = 4194439
	KeyKP2          = 4194440
	KeyKP3          = 4194441
	KeyKP4          = 4194442
	KeyKP5          = 4194443
	KeyKP6          = 4194444
	KeyKP7          = 4194445
	KeyKP8          = 4194446
	KeyKP9          = 4194447
	KeyMenu         = 4194370
	KeyHyper        = 4194371
	KeyHelp         = 4194373
	KeyBack         = 4194376
	KeyForward      = 4194377
	KeyStop         = 4194378
	KeyRefresh      = 4194379
	KeyVolumeDown   = 4194380
	KeyVolumeMute   = 4194381
	KeyVolumeUp     = 4194382
	KeyMediaPlay    = 4194388
	KeyMediaStop    = 4194389
	KeyMediaPrev    = 4194390
	KeyMediaNext    = 4194391
	KeyMediaRecord  = 4194392
	KeyHomePage     = 4194393
	KeyFavorites    = 4194394
	KeySearch       = 4194395
	KeyStandby      = 4194396
	KeyOpenURL      = 4194397
	KeyLaunchMail   = 4194398
	KeyLaunchMedia  = 4194399
	KeyLaunch0      = 4194400
	KeyLaunch1      = 4194401
	KeyLaunch2      = 4194402
	KeyLaunch3      = 4194403
	KeyLaunch4      = 4194404
	KeyLaunch5      = 4194405
	KeyLaunch6      = 4194406
	KeyLaunch7      = 4194407
	KeyLaunch8      = 4194408
	KeyLaunch9      = 4194409
	KeyLaunchA      = 4194410
	KeyLaunchB      = 4194411
	KeyLaunchC      = 4194412
	KeyLaunchD      = 4194413
	KeyLaunchE      = 4194414
	KeyLaunchF      = 4194415
	KeyGlobe        = 4194416
	KeyKeyboard     = 4194417
	KeyJISEisu      = 4194418
	KeyJISKana      = 4194419
	KeyUnknown      = 8388607
	KeySpace        = 32
	KeyExclam       = 33
	KeyQuoteDbl     = 34
	KeyNumberSign   = 35
	KeyDollar       = 36
	KeyPercent      = 37
	KeyAmpersand    = 38
	KeyApostrophe   = 39
	KeyParenLeft    = 40
	KeyParenRight   = 41
	KeyAsterisk     = 42
	KeyPlus         = 43
	KeyComma        = 44
	KeyMinus        = 45
	KeyPeriod       = 46
	KeySlash        = 47
	Key0            = 48
	Key1            = 49
	Key2            = 50
	Key3            = 51
	Key4            = 52
	Key5            = 53
	Key6            = 54
	Key7            = 55
	Key8            = 56
	Key9            = 57
	KeyColon        = 58
	KeySemicolon    = 59
	KeyLess         = 60
	KeyEqual        = 61
	KeyGreater      = 62
	KeyQuestion     = 63
	KeyAt           = 64
	KeyA            = 65
	KeyB            = 66
	KeyC            = 67
	KeyD            = 68
	KeyE            = 69
	KeyF            = 70
	KeyG            = 71
	KeyH            = 72
	KeyI            = 73
	KeyJ            = 74
	KeyK            = 75
	KeyL            = 76
	KeyM            = 77
	KeyN            = 78
	KeyO            = 79
	KeyP            = 80
	KeyQ            = 81
	KeyR            = 82
	KeyS            = 83
	KeyT            = 84
	KeyU            = 85
	KeyV            = 86
	KeyW            = 87
	KeyX            = 88
	KeyY            = 89
	KeyZ            = 90
	KeyBracketLeft  = 91
	KeyBackslash    = 92
	KeyBracketRight = 93
	KeyAsciiCircum  = 94
	KeyUnderscore   = 95
	KeyQuoteLeft    = 96
	KeyBraceLeft    = 123
	KeyBar          = 124
	KeyBraceRight   = 125
	KeyAsciiTilde   = 126
	KeyYen          = 165
	KeySection      = 167
	KeyGraveAccent  = KeyQuoteLeft
	KeyKPDecimal    = KeyKPPeriod
	KeyKPEqual      = KeyEqual
	KeyPrintScreen  = KeyPrint
	KeyLeftBracket  = KeyBracketLeft
	KeyRightBracket = KeyBracketRight
	KeyControl      = KeyCmdOrCtrl
)
