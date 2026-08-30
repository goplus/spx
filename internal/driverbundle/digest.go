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

package driverbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ComputeEngineInterfaceDigest identifies an Engine/PCK pair.
func ComputeEngineInterfaceDigest(engine, pack []byte) string {
	engineDigest := sha256.Sum256(engine)
	packDigest := sha256.Sum256(pack)
	return computeEngineInterfaceDigest(engineDigest, packDigest)
}

// ComputeEngineInterfaceDigestFromSHA256 identifies an Engine/PCK pair from
// their verified content digests.
func ComputeEngineInterfaceDigestFromSHA256(engine, pack string) (string, error) {
	engineDigest, err := decodeSHA256(engine)
	if err != nil {
		return "", fmt.Errorf("invalid Engine SHA-256: %w", err)
	}
	packDigest, err := decodeSHA256(pack)
	if err != nil {
		return "", fmt.Errorf("invalid PCK SHA-256: %w", err)
	}
	return computeEngineInterfaceDigest(engineDigest, packDigest), nil
}

func decodeSHA256(value string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if err := validateSHA256(value); err != nil {
		return digest, err
	}
	_, err := hex.Decode(digest[:], []byte(value))
	return digest, err
}

func computeEngineInterfaceDigest(engine, pack [sha256.Size]byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(EngineInterfaceDigestDomain))
	hasher.Write(engine[:])
	hasher.Write(pack[:])
	return hex.EncodeToString(hasher.Sum(nil))
}
