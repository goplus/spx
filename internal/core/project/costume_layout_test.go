package project

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrepareCostumeLayout(t *testing.T) {
	tests := []struct {
		name       string
		config     SpriteConfig
		wantFrames []CostumeFrame
		wantNames  map[string]int
	}{
		{
			name: "costume list preserves first duplicate name",
			config: SpriteConfig{Costumes: []*CostumeConfig{
				{Name: "idle"}, {Name: "idle"}, {Name: "run"},
			}},
			wantFrames: []CostumeFrame{
				{Name: "idle", Index: 0},
				{Name: "idle", Index: 1},
				{Name: "run", Index: 2},
			},
			wantNames: map[string]int{"idle": 0, "run": 2},
		},
		{
			name: "single frame ignores items",
			config: SpriteConfig{CostumeSet: &CostumeSet{
				Nx: 1, Items: []CostumeSetItem{{NamePrefix: "ignored", N: -1}},
			}},
			wantFrames: []CostumeFrame{{Name: "0"}},
			wantNames:  map[string]int{"0": 0},
		},
		{
			name: "named frames",
			config: SpriteConfig{CostumeSet: &CostumeSet{
				Nx: 3, Items: []CostumeSetItem{{NamePrefix: "idle", N: 1}, {NamePrefix: "run", N: 2}},
			}},
			wantFrames: []CostumeFrame{
				{Name: "idle0"},
				{Name: "run0", Index: 1},
				{Name: "run1", Index: 2},
			},
			wantNames: map[string]int{"idle0": 0, "run0": 1, "run1": 2},
		},
		{
			name: "multipart default names use global indexes",
			config: SpriteConfig{CostumeMPSet: &CostumeMPSet{Parts: []CostumeSetPart{
				{Nx: 1}, {Nx: 2},
			}}},
			wantFrames: []CostumeFrame{
				{Name: "0"},
				{Name: "1", Part: 1},
				{Name: "2", Part: 1, Index: 1},
			},
			wantNames: map[string]int{"0": 0, "1": 1, "2": 2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout, err := PrepareCostumeLayout(&test.config)
			if err != nil {
				t.Fatalf("PrepareCostumeLayout() error = %v", err)
			}
			if !reflect.DeepEqual(layout.Frames, test.wantFrames) {
				t.Fatalf("Frames = %#v, want %#v", layout.Frames, test.wantFrames)
			}
			if !reflect.DeepEqual(layout.nameIndexes, test.wantNames) {
				t.Fatalf("nameIndexes = %#v, want %#v", layout.nameIndexes, test.wantNames)
			}
		})
	}
}

func TestPrepareCostumeLayoutRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name      string
		config    *SpriteConfig
		wantError string
	}{
		{name: "nil config", wantError: "configuration is nil"},
		{name: "no declaration", config: &SpriteConfig{}, wantError: "must define"},
		{name: "empty list", config: &SpriteConfig{Costumes: []*CostumeConfig{}}, wantError: "must not be empty"},
		{name: "null costume", config: &SpriteConfig{Costumes: []*CostumeConfig{nil}}, wantError: "costumes[0] is null"},
		{name: "empty multipart", config: &SpriteConfig{CostumeMPSet: &CostumeMPSet{}}, wantError: "parts must not be empty"},
		{name: "invalid count", config: &SpriteConfig{CostumeSet: &CostumeSet{}}, wantError: "invalid frame count 0"},
		{
			name: "negative item count",
			config: &SpriteConfig{CostumeSet: &CostumeSet{
				Nx: 2, Items: []CostumeSetItem{{N: -1}},
			}},
			wantError: "negative frame count -1",
		},
		{
			name: "incomplete item count",
			config: &SpriteConfig{CostumeSet: &CostumeSet{
				Nx: 2, Items: []CostumeSetItem{{N: 1}},
			}},
			wantError: "incomplete frame loading",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := PrepareCostumeLayout(test.config)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("PrepareCostumeLayout() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestCostumeLayoutResolveFrameIndex(t *testing.T) {
	type frameIndex int

	layout, err := PrepareCostumeLayout(&SpriteConfig{
		Costumes: []*CostumeConfig{{Name: "idle"}, {Name: "run"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		value     any
		wantIndex int
		wantFound bool
	}{
		{name: "costume name", value: "run", wantIndex: 1, wantFound: true},
		{name: "missing name", value: "missing", wantFound: false},
		{name: "numeric index", value: 1.9, wantIndex: 1, wantFound: true},
		{name: "named numeric type uses runtime fallback", value: frameIndex(1), wantIndex: 0, wantFound: true},
		{name: "unsupported type uses runtime fallback", value: struct{}{}, wantIndex: 0, wantFound: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, found := layout.ResolveFrameIndex(test.value)
			if index != test.wantIndex || found != test.wantFound {
				t.Fatalf("ResolveFrameIndex(%v) = (%d, %v), want (%d, %v)", test.value, index, found, test.wantIndex, test.wantFound)
			}
		})
	}
}
