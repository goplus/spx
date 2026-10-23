//go:build unix || windows

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

package engine

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
)

func runTrackedEngineCommandWithEnv(workdir string, env map[string]string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if len(env) != 0 {
		cmd.Env = shared.EnvMapToSlice(env)
	}
	configureTrackedCommand(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, trackedSignals...)
	defer signal.Stop(sigCh)

	select {
	case err := <-done:
		return err
	case <-sigCh:
		terminateTrackedProcessGroup(cmd.Process)
		err := <-done
		if err != nil {
			return err
		}
		return errors.New("engine build interrupted")
	}
}
