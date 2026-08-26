/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package launchpack

import (
	"encoding/json"
	"errors"
)

type componentDigests struct {
	interfaceDigest string
	engine          string
	pack            string
	bridge          string
}

type driverManifestFields struct {
	DriverManifestSHA256 string `json:"driver_manifest_sha256,omitempty"`
	DriverBundleSHA256   string `json:"driver_bundle_sha256,omitempty"`
	DriverBundleName     string `json:"driver_bundle_name,omitempty"`
	DriverSPXVersion     string `json:"driver_spx_version,omitempty"`
}

type engineComponentManifest struct {
	driverManifestFields
	Schema                string `json:"schema"`
	Mode                  string `json:"mode"`
	RuntimeVersion        string `json:"runtime_version"`
	RuntimeABI            int    `json:"runtime_abi"`
	EngineInterfaceDigest string `json:"engine_interface_digest"`
	ExecutableSHA256      string `json:"executable_sha256"`
	PackSHA256            string `json:"pack_sha256"`
}

type bridgeComponentManifest struct {
	driverManifestFields
	Schema                string `json:"schema"`
	Mode                  string `json:"mode"`
	SPXSource             string `json:"spx_source"`
	EngineInterfaceDigest string `json:"engine_interface_digest"`
	BridgeSHA256          string `json:"bridge_sha256"`
}

func componentManifests(cfg Config, assets Assets, digests componentDigests) ([]byte, []byte, error) {
	mode, engineSchema, bridgeSchema := "source", "spx-local-engine/v1", "spx-local-bridge/v1"
	var driver driverManifestFields
	if cfg.Source.SourceMode {
		if assets.Published != nil {
			return nil, nil, errors.New("launchpack: source assets carry published driver provenance")
		}
	} else {
		if assets.Published == nil {
			return nil, nil, errors.New("launchpack: published driver provenance is incomplete")
		}
		if err := assets.Published.validate(); err != nil {
			return nil, nil, err
		}
		if err := assets.Published.verifyDigests(digests.engine, digests.pack, digests.bridge, digests.interfaceDigest); err != nil {
			return nil, nil, err
		}
		mode, engineSchema, bridgeSchema = "published", "spx-published-engine/v1", "spx-published-bridge/v1"
		driver = driverManifestFields{
			DriverManifestSHA256: assets.Published.ManifestSHA256,
			DriverBundleSHA256:   assets.Published.BundleSHA256,
			DriverBundleName:     assets.Published.BundleName,
			DriverSPXVersion:     assets.Published.SPXVersion,
		}
	}
	engine, err := json.Marshal(engineComponentManifest{
		driverManifestFields: driver,
		Schema:               engineSchema, Mode: mode, RuntimeVersion: assets.Lock.RuntimeVersion,
		RuntimeABI: assets.Lock.RuntimeABI, EngineInterfaceDigest: digests.interfaceDigest,
		ExecutableSHA256: digests.engine, PackSHA256: digests.pack,
	})
	if err != nil {
		return nil, nil, err
	}
	bridge, err := json.Marshal(bridgeComponentManifest{
		driverManifestFields: driver,
		Schema:               bridgeSchema, Mode: mode, SPXSource: cfg.Source.EffectivePath,
		EngineInterfaceDigest: digests.interfaceDigest, BridgeSHA256: digests.bridge,
	})
	return engine, bridge, err
}
