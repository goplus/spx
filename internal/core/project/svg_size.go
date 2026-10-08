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

package project

import (
	"encoding/xml"
	"io"
	"math"
	"path"
	"strconv"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
)

// fillMissingSVGCostumeSizes preserves fractional SVG dimensions for geometry calculations.
func fillMissingSVGCostumeSizes(fs spxfs.Dir, costumes []*CostumeConfig) {
	for _, costume := range costumes {
		if costume == nil || (costume.ImageWidth > 0 && costume.ImageHeight > 0) {
			continue
		}
		if !strings.EqualFold(path.Ext(costume.Path), ".svg") {
			continue
		}

		file, err := fs.Open(costume.Path)
		if err != nil {
			// Keep the texture-size fallback when SVG metadata is unavailable.
			continue
		}
		width, height, ok := readSVGLogicalSize(file)
		_ = file.Close()
		if !ok {
			continue
		}
		if costume.ImageWidth <= 0 {
			costume.ImageWidth = width
		}
		if costume.ImageHeight <= 0 {
			costume.ImageHeight = height
		}
	}
}

// readSVGLogicalSize reads intrinsic dimensions and uses the viewBox aspect ratio for omitted dimensions.
func readSVGLogicalSize(r io.Reader) (width, height float64, ok bool) {
	decoder := xml.NewDecoder(r)
	for {
		token, err := decoder.Token()
		if err != nil {
			return 0, 0, false
		}
		start, isStart := token.(xml.StartElement)
		if !isStart || start.Name.Local != "svg" {
			continue
		}

		var widthText, heightText, viewBoxText string
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "width":
				widthText = attr.Value
			case "height":
				heightText = attr.Value
			case "viewBox":
				viewBoxText = attr.Value
			}
		}

		width, widthState := parseSVGLength(widthText)
		height, heightState := parseSVGLength(heightText)
		if widthState == svgLengthUnsupported || heightState == svgLengthUnsupported {
			// Let the renderer resolve units that are not handled here.
			return 0, 0, false
		}
		if widthState == svgLengthValid && heightState == svgLengthValid {
			return width, height, true
		}

		viewBoxWidth, viewBoxHeight, viewBoxOK := parseSVGViewBox(viewBoxText)
		if !viewBoxOK {
			return 0, 0, false
		}
		switch {
		case widthState == svgLengthValid:
			height = width * viewBoxHeight / viewBoxWidth
		case heightState == svgLengthValid:
			width = height * viewBoxWidth / viewBoxHeight
		default:
			width, height = viewBoxWidth, viewBoxHeight
		}
		if math.IsInf(width, 0) || math.IsInf(height, 0) {
			return 0, 0, false
		}
		return width, height, true
	}
}

type svgLengthState uint8

const (
	svgLengthMissing svgLengthState = iota
	svgLengthValid
	svgLengthUnsupported
)

func parseSVGLength(value string) (float64, svgLengthState) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, svgLengthMissing
	}
	value = strings.TrimSpace(strings.TrimSuffix(value, "px"))
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, svgLengthUnsupported
	}
	return parsed, svgLengthValid
}

func parseSVGViewBox(value string) (width, height float64, ok bool) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	})
	if len(fields) != 4 {
		return 0, 0, false
	}
	width, widthState := parseSVGLength(fields[2])
	height, heightState := parseSVGLength(fields[3])
	return width, height, widthState == svgLengthValid && heightState == svgLengthValid
}
