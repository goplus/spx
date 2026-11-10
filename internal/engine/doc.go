// Package engine adapts SPX scripts and runtime objects to a single active
// engine session. Process configuration, such as type registrations and capture
// handlers, survives session cleanup; objects and pending frame work do not.
//
// Execute and Go run managed script work. ExecuteNative yields that script while
// native work runs, and WaitMainThread dispatches an entire engine operation.
// Managers dispatches individual backend calls; compound operations and runtime
// registry updates still require the caller's serialized execution context.
// Object methods marked as main-thread operations must run on the engine thread.
package engine
