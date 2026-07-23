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

package spx

import (
	"fmt"
	"sync"

	"github.com/goplus/spx/v3/internal/engine"
	inputstate "github.com/goplus/spx/v3/internal/input"
)

const defaultInputSessionRandomSeed int64 = 1

// inputSessionPlan is an immutable, one-shot descriptor for the next game
// lifecycle. Runtime state is created only after a Game claims the plan.
type inputSessionPlan struct {
	token         uint64
	mode          InputSessionMode
	fixedTimestep float64
	replay        InputReplay
	captureKey    Key
}

var preparedInputSession struct {
	sync.Mutex
	nextToken uint64
	plan      *inputSessionPlan
	claimed   bool
}

// inputSession belongs to exactly one Game bootstrap generation.
type inputSession struct {
	operationMu sync.Mutex
	mu          sync.Mutex

	generation uint64
	mode       InputSessionMode
	phase      InputSessionPhase
	captureKey Key
	controller inputstate.InputReplayController

	initial        InputReplayState
	current        InputReplayState
	currentTime    float64
	currentTick    int64
	hasCurrentTick bool
	initialPending bool
	result         InputReplay
	resultJSON     []byte
	abortReason    string
	frameOpen      bool
	terminalStatus InputSessionStatus
	hasTerminal    bool

	input       inputSessionInput
	environment inputSessionEnvironment
}

type inputRecordingResult struct {
	replay InputReplay
	json   []byte
}

func prepareInputRecordingSession(fixedTimestep float64, option InputSessionOptions) (InputSessionPreparation, error) {
	return prepareInputSession(inputSessionPlan{
		mode:          InputSessionModeRecording,
		fixedTimestep: fixedTimestep,
		captureKey:    option.CaptureKey,
	})
}

func prepareInputReplaySession(replay InputReplay, option InputSessionOptions) (InputSessionPreparation, error) {
	if err := replay.Validate(); err != nil {
		return InputSessionPreparation{}, err
	}
	return prepareInputSession(inputSessionPlan{
		mode:       InputSessionModeReplaying,
		replay:     cloneInputReplay(replay),
		captureKey: option.CaptureKey,
	})
}

func prepareInputSession(plan inputSessionPlan) (InputSessionPreparation, error) {
	preparedInputSession.Lock()
	defer preparedInputSession.Unlock()
	if preparedInputSession.plan != nil {
		return InputSessionPreparation{}, ErrInputSessionActive
	}
	if game := activeGame(); game != nil {
		if game.inputSessionUnavailable() {
			return InputSessionPreparation{}, ErrInputSessionActive
		}
	}
	preparedInputSession.nextToken++
	if preparedInputSession.nextToken == 0 {
		preparedInputSession.nextToken++
	}
	plan.token = preparedInputSession.nextToken
	preparedInputSession.plan = &plan
	preparedInputSession.claimed = false
	return InputSessionPreparation{token: plan.token}, nil
}

func claimPreparedInputSession() *inputSessionPlan {
	preparedInputSession.Lock()
	defer preparedInputSession.Unlock()
	if preparedInputSession.plan == nil || preparedInputSession.claimed {
		return nil
	}
	preparedInputSession.claimed = true
	return preparedInputSession.plan
}

func completePreparedInputSessionClaim(plan *inputSessionPlan) {
	preparedInputSession.Lock()
	if preparedInputSession.plan == plan {
		preparedInputSession.plan = nil
		preparedInputSession.claimed = false
	}
	preparedInputSession.Unlock()
}

func cancelPreparedInputSession(token uint64) bool {
	preparedInputSession.Lock()
	defer preparedInputSession.Unlock()
	if token == 0 || preparedInputSession.plan == nil || preparedInputSession.claimed || preparedInputSession.plan.token != token {
		return false
	}
	preparedInputSession.plan = nil
	return true
}

func clearPreparedInputSession() {
	preparedInputSession.Lock()
	preparedInputSession.plan = nil
	preparedInputSession.claimed = false
	preparedInputSession.Unlock()
}

func preparedInputSessionStatus() (InputSessionStatus, bool) {
	preparedInputSession.Lock()
	defer preparedInputSession.Unlock()
	if preparedInputSession.plan == nil {
		return InputSessionStatus{}, false
	}
	status := InputSessionStatus{
		Mode:  preparedInputSession.plan.mode,
		Phase: InputSessionPhasePrepared,
	}
	if status.Mode == InputSessionModeReplaying {
		status.FrameCount = len(preparedInputSession.plan.replay.Frames)
	}
	return status, true
}

func newInputSession(plan *inputSessionPlan, generation uint64) (*inputSession, error) {
	session := &inputSession{
		generation: generation,
		mode:       plan.mode,
		phase:      InputSessionPhaseRunning,
		captureKey: plan.captureKey,
	}

	var fixedTimestep float64
	switch plan.mode {
	case InputSessionModeRecording:
		fixedTimestep = plan.fixedTimestep
		if err := session.controller.StartRecording(InputReplayState{}, fixedTimestep); err != nil {
			return nil, err
		}
		session.initialPending = true
	case InputSessionModeReplaying:
		if err := session.controller.StartReplay(plan.replay); err != nil {
			return nil, err
		}
		fixedTimestep = plan.replay.FixedTimestep
		session.initial = cloneInputReplayState(plan.replay.Initial)
		session.current = cloneInputReplayState(plan.replay.Initial)
	default:
		return nil, fmt.Errorf("unsupported input session mode %q", plan.mode)
	}

	engine.DiscardPendingKeyEvents()
	session.environment.start(fixedTimestep)
	setDeterministicRandomSeed(defaultInputSessionRandomSeed)
	engine.SetMouseEventCaptureEnabled(true)
	return session, nil
}

func (p *Game) attachPreparedInputSession() error {
	p.inputSessionMu.Lock()
	if p.inputClaimed || p.inputSession != nil {
		p.inputSessionMu.Unlock()
		return ErrInputSessionActive
	}
	if p.lifecycleState.IsRunned.Load() {
		p.inputSessionMu.Unlock()
		return fmt.Errorf("input session must be attached before the game starts")
	}
	p.inputClaimed = true
	p.hasInputTerm = false
	p.inputTerminal = InputSessionStatus{}
	p.inputSessionMu.Unlock()

	plan := claimPreparedInputSession()
	if plan == nil {
		return nil
	}
	session, err := newInputSession(plan, p.currentBootstrapGeneration())
	if err != nil {
		p.setInputSessionTerminal(InputSessionStatus{
			Mode:  plan.mode,
			Phase: InputSessionPhaseAborted,
			Error: err.Error(),
		})
		completePreparedInputSessionClaim(plan)
		return err
	}
	p.inputSessionMu.Lock()
	if !p.inputClaimed || p.inputSession != nil {
		p.inputSessionMu.Unlock()
		p.setInputSessionTerminal(session.close("another input session is already attached"))
		completePreparedInputSessionClaim(plan)
		return fmt.Errorf("game lifecycle ended before input session attachment completed")
	}
	p.inputSession = session
	p.inputSessionMu.Unlock()
	completePreparedInputSessionClaim(plan)
	return nil
}

func (p *Game) inputSessionUnavailable() bool {
	if p == nil {
		return false
	}
	p.inputSessionMu.RLock()
	unavailable := p.inputClaimed || p.lifecycleState.IsRunned.Load()
	p.inputSessionMu.RUnlock()
	return unavailable
}

func (p *Game) currentInputSession() *inputSession {
	if p == nil {
		return nil
	}
	p.inputSessionMu.RLock()
	session := p.inputSession
	p.inputSessionMu.RUnlock()
	return session
}

func (p *Game) abortInputSession(reason string) {
	if p == nil {
		return
	}
	p.inputSessionMu.Lock()
	if p.inputSession != nil {
		p.inputTerminal = p.inputSession.close(reason)
		p.hasInputTerm = true
		p.inputSession = nil
	}
	p.inputClaimed = false
	p.inputSessionMu.Unlock()
}

func (p *Game) setInputSessionTerminal(status InputSessionStatus) {
	p.inputSessionMu.Lock()
	p.inputSession = nil
	p.inputTerminal = status
	p.hasInputTerm = true
	p.inputSessionMu.Unlock()
}

func (p *Game) terminalInputSessionStatus() (InputSessionStatus, bool) {
	p.inputSessionMu.RLock()
	status, ok := p.inputTerminal, p.hasInputTerm
	p.inputSessionMu.RUnlock()
	return status, ok
}

func (s *inputSession) close(reason string) InputSessionStatus {
	s.operationMu.Lock()
	s.mu.Lock()
	if s.phase != InputSessionPhaseCompleted && s.phase != InputSessionPhaseAborted {
		s.phase = InputSessionPhaseAborted
		s.abortReason = reason
		s.result = InputReplay{}
		s.resultJSON = nil
	}
	status := s.statusLocked()
	s.terminalStatus = status
	s.hasTerminal = true
	s.controller.Reset()
	s.mu.Unlock()
	s.operationMu.Unlock()

	s.environment.stop()
	ResetRandomSeed()
	engine.DiscardPendingKeyEvents()
	engine.SetMouseEventCaptureEnabled(false)
	return status
}

func finishInputRecordingResultSession() (inputRecordingResult, error) {
	game := activeGame()
	if game == nil {
		return inputRecordingResult{}, ErrInputSessionNotRecording
	}
	session := game.currentInputSession()
	if session == nil {
		return inputRecordingResult{}, ErrInputSessionNotRecording
	}
	var freeze func()
	if game.lifecycleState.IsRunned.Load() {
		freeze = func() { game.engine().ExtMgr.Pause() }
	}
	result, err := session.finishRecordingResult(freeze)
	if err != nil && freeze != nil {
		// Finish is the host's Game-stop boundary. Even an uncommittable
		// recording must be quiesced so capture draining cannot be starved by
		// later frames while shutdown reports the original error.
		_ = freezeInputSession(freeze)
	}
	return result, err
}

func finishInputRecordingSession() (InputReplay, error) {
	result, err := finishInputRecordingResultSession()
	return result.replay, err
}

func finishInputRecordingJSONSession() (string, error) {
	result, err := finishInputRecordingResultSession()
	return string(result.json), err
}

func (s *inputSession) finishRecording(freeze func()) (InputReplay, error) {
	result, err := s.finishRecordingResult(freeze)
	return result.replay, err
}

func (s *inputSession) finishRecordingResult(freeze func()) (inputRecordingResult, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	s.mu.Lock()
	if s.mode != InputSessionModeRecording {
		s.mu.Unlock()
		return inputRecordingResult{}, ErrInputSessionNotRecording
	}
	if s.phase == InputSessionPhaseAborted {
		reason := s.abortReason
		s.mu.Unlock()
		return inputRecordingResult{}, fmt.Errorf("input recording was aborted: %s", reason)
	}
	if s.phase == InputSessionPhaseCompleted {
		result := inputRecordingResult{
			replay: cloneInputReplay(s.result),
			json:   append([]byte(nil), s.resultJSON...),
		}
		s.mu.Unlock()
		return result, nil
	}
	if s.phase != InputSessionPhaseRunning {
		phase := s.phase
		s.mu.Unlock()
		return inputRecordingResult{}, fmt.Errorf("input recording cannot finish in phase %q", phase)
	}
	if s.frameOpen {
		s.mu.Unlock()
		return inputRecordingResult{}, fmt.Errorf("input recording cannot finish before the current frame ends")
	}

	s.phase = InputSessionPhaseFinishing
	replay, err := s.controller.Recording()
	if err != nil {
		s.phase = InputSessionPhaseRunning
		s.mu.Unlock()
		return inputRecordingResult{}, err
	}
	encoded, err := inputstate.EncodeInputReplay(replay)
	if err != nil {
		s.phase = InputSessionPhaseRunning
		s.mu.Unlock()
		return inputRecordingResult{}, err
	}
	s.result = cloneInputReplay(replay)
	s.resultJSON = append(s.resultJSON[:0], encoded...)
	s.mu.Unlock()

	if err := freezeInputSession(freeze); err != nil {
		s.mu.Lock()
		s.result = InputReplay{}
		s.resultJSON = nil
		s.phase = InputSessionPhaseRunning
		s.mu.Unlock()
		return inputRecordingResult{}, err
	}
	s.mu.Lock()
	s.phase = InputSessionPhaseCompleted
	s.mu.Unlock()
	return inputRecordingResult{replay: replay, json: encoded}, nil
}

func freezeInputSession(freeze func()) (err error) {
	if freeze == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("freeze input session: %v", recovered)
		}
	}()
	freeze()
	return nil
}

// resetInputSessionState is retained as an internal cleanup seam. Product
// callers end the Game instead of resetting input independently.
func resetInputSessionState() {
	clearPreparedInputSession()
	if game := activeGame(); game != nil {
		game.abortInputSession("input session reset")
		game.inputSessionMu.Lock()
		game.inputTerminal = InputSessionStatus{}
		game.hasInputTerm = false
		game.inputSessionMu.Unlock()
	}
}

func activeInputSession() *inputSession {
	if game := activeGame(); game != nil {
		return game.currentInputSession()
	}
	return nil
}

func inputSessionStatus() InputSessionStatus {
	// The prepared descriptor remains visible until its Game-owned session is
	// installed. Reading it first makes the handoff monotonic: Prepared never
	// briefly appears as Idle while startup polling is in progress.
	if status, ok := preparedInputSessionStatus(); ok {
		return status
	}
	if session := activeInputSession(); session != nil {
		return session.status()
	}
	if game := activeGame(); game != nil {
		if status, ok := game.terminalInputSessionStatus(); ok {
			return status
		}
	}
	return InputSessionStatus{Mode: InputSessionModeIdle}
}

func (s *inputSession) status() InputSessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *inputSession) statusLocked() InputSessionStatus {
	if s.hasTerminal {
		return s.terminalStatus
	}
	controllerStatus := s.controller.Status()
	status := InputSessionStatus{
		Mode:       s.mode,
		Phase:      s.phase,
		Completed:  s.phase == InputSessionPhaseCompleted,
		Exhausted:  controllerStatus.Exhausted,
		NextFrame:  controllerStatus.NextFrame,
		FrameCount: controllerStatus.FrameCount,
	}
	if s.hasCurrentTick {
		status.CurrentTick = s.currentTick
		status.HasCurrentTick = true
	}
	if s.mode == InputSessionModeRecording && s.phase == InputSessionPhaseCompleted {
		status.NextFrame = int64(len(s.result.Frames))
		status.FrameCount = len(s.result.Frames)
	}
	if s.phase == InputSessionPhaseAborted {
		status.Error = s.abortReason
	}
	return status
}

func (s *inputSession) beginFrame() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != InputSessionPhaseRunning || s.frameOpen {
		return false
	}
	s.frameOpen = true
	return true
}

func (s *inputSession) endFrame() {
	s.mu.Lock()
	s.frameOpen = false
	s.mu.Unlock()
}

func (s *inputSession) frameCompletionPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode == InputSessionModeReplaying && s.phase == InputSessionPhaseFinishing
}

func (p *Game) inputSessionFrameCompletionPending() bool {
	session := p.currentInputSession()
	return session != nil && session.frameCompletionPending()
}

func (p *Game) finishInputSessionFrame() {
	session := p.currentInputSession()
	if session == nil {
		return
	}
	session.endFrame()
	completed, err := session.completeReplayFrame(func() { p.engine().ExtMgr.Pause() })
	if err != nil {
		engine.Panic(err)
		return
	}
	if !completed {
		return
	}
}

func (s *inputSession) completeReplayFrame(freeze func()) (bool, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	s.mu.Lock()
	if s.mode != InputSessionModeReplaying || s.phase != InputSessionPhaseFinishing {
		s.mu.Unlock()
		return false, nil
	}
	if !s.controller.Status().Exhausted {
		s.mu.Unlock()
		return false, nil
	}
	s.mu.Unlock()

	if err := freezeInputSession(freeze); err != nil {
		s.mu.Lock()
		s.phase = InputSessionPhaseAborted
		s.abortReason = err.Error()
		s.mu.Unlock()
		return false, err
	}
	s.mu.Lock()
	s.phase = InputSessionPhaseCompleted
	s.mu.Unlock()
	return true, nil
}

func cloneInputReplay(replay InputReplay) InputReplay {
	cloned := replay
	cloned.Initial = cloneInputReplayState(replay.Initial)
	cloned.Frames = make([]InputReplayFrame, len(replay.Frames))
	for i, frame := range replay.Frames {
		cloned.Frames[i] = frame
		cloned.Frames[i].State = cloneInputReplayState(frame.State)
		cloned.Frames[i].MouseEvents = append([]InputReplayMouseEvent(nil), frame.MouseEvents...)
		cloned.Frames[i].KeyEvents = append([]InputReplayKeyEvent(nil), frame.KeyEvents...)
	}
	return cloned
}
